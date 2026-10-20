package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/containernetworking/plugins/pkg/ns"
	goipam "github.com/metal-stack/go-ipam"
	"github.com/vishvananda/netlink"
)

const DefaultStateDir = "/var/lib/cni/zero-copy"

type IPAM struct {
	Ipam    goipam.Ipamer
	PodCIDR string
	Prefix  *goipam.Prefix

	ipNet    *net.IPNet
	gateway  net.IP
	stateDir string
	lockFile *os.File
}

// NewIPAM builds the allocator for this node's podCIDR (e.g. 10.244.1.0/24),
// takes the node-wide lock, reserves the gateway and replays existing allocations.
// The caller MUST call Close() when finished.
func NewIPAM(ctx context.Context, ipamer goipam.Ipamer, podCIDR, stateDir string) (*IPAM, error) {
	_, ipNet, err := net.ParseCIDR(podCIDR)
	if err != nil {
		return nil, fmt.Errorf("parse pod CIDR %q: %w", podCIDR, err)
	}
	if ipNet.IP.To4() == nil {
		return nil, fmt.Errorf("pod CIDR %q is not IPv4", podCIDR)
	}
	if stateDir == "" {
		stateDir = DefaultStateDir
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}
	lock, err := os.OpenFile(filepath.Join(stateDir, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		lock.Close()
		return nil, fmt.Errorf("lock state dir: %w", err)
	}

	i := &IPAM{
		Ipam:     ipamer,
		PodCIDR:  ipNet.String(),
		ipNet:    ipNet,
		stateDir: stateDir,
		lockFile: lock,
	}
	if err := i.init(ctx); err != nil {
		i.Close()
		return nil, err
	}
	return i, nil
}

func (i *IPAM) init(ctx context.Context) error {
	prefix, err := i.Ipam.PrefixFrom(ctx, i.PodCIDR)
	if err != nil {
		if !errors.Is(err, goipam.ErrNotFound) {
			return fmt.Errorf("lookup prefix: %w", err)
		}
		if prefix, err = i.Ipam.NewPrefix(ctx, i.PodCIDR); err != nil {
			return fmt.Errorf("create prefix: %w", err)
		}
	}
	i.Prefix = prefix

	gw, err := firstUsableIP(prefix.Cidr)
	if err != nil {
		return err
	}
	i.gateway = gw
	// Reserve the gateway so it is never handed to a pod.
	if _, err := i.Ipam.AcquireSpecificIP(ctx, prefix.Cidr, gw.String()); err != nil &&
		!errors.Is(err, goipam.ErrAlreadyAllocated) {
		return fmt.Errorf("reserve gateway %s: %w", gw, err)
	}

	// Replay allocations of running pods (needed for the in-memory backend).
	entries, err := os.ReadDir(i.stateDir)
	if err != nil {
		return fmt.Errorf("read state dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		addr, ok, err := readRecord(filepath.Join(i.stateDir, e.Name()))
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if _, err := i.Ipam.AcquireSpecificIP(ctx, prefix.Cidr, addr.String()); err != nil &&
			!errors.Is(err, goipam.ErrAlreadyAllocated) {
			return fmt.Errorf("replay %s: %w", addr, err)
		}
	}
	return nil
}

// Close releases the node-wide lock.
func (i *IPAM) Close() error {
	if i == nil || i.lockFile == nil {
		return nil
	}
	_ = syscall.Flock(int(i.lockFile.Fd()), syscall.LOCK_UN)
	err := i.lockFile.Close()
	i.lockFile = nil
	return err
}

// Gateway returns the first usable address of the pod CIDR.
func (i *IPAM) Gateway() (net.IP, error) {
	if i == nil || i.gateway == nil {
		return nil, fmt.Errorf("IPAM is not initialised")
	}
	return i.gateway, nil
}

// GatewayCIDR returns the gateway with the pod mask (e.g. 10.244.1.1/24), for the bridge.
func (i *IPAM) GatewayCIDR() (*net.IPNet, error) {
	gw, err := i.Gateway()
	if err != nil {
		return nil, err
	}
	return &net.IPNet{IP: gw, Mask: i.ipNet.Mask}, nil
}

func firstUsableIP(cidr string) (net.IP, error) {
	ip, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse network CIDR %q: %w", cidr, err)
	}
	ip = ip.Mask(network.Mask)
	if ip.To4() == nil {
		return nil, fmt.Errorf("network CIDR %q is not IPv4", cidr)
	}
	prefixSize, addressSize := network.Mask.Size()
	if prefixSize >= addressSize-1 {
		return nil, fmt.Errorf("network CIDR %q is too small", cidr)
	}
	gateway := append(net.IP(nil), ip.To4()...)
	for index := len(gateway) - 1; index >= 0; index-- {
		gateway[index]++
		if gateway[index] != 0 {
			return gateway, nil
		}
	}
	return nil, fmt.Errorf("network CIDR %q has no usable gateway", cidr)
}

// AllocateIP assigns an address to ifName inside the pod netns and returns it
// (with the pod mask) so the caller can put it in the CNI result. Idempotent per containerID+ifName.
func (i *IPAM) AllocateIP(ctx context.Context, containerID, netnsPath, ifName string) (*net.IPNet, error) {
	recPath, err := i.recordPath(containerID, ifName)
	if err != nil {
		return nil, err
	}

	addr, found, err := readRecord(recPath)
	if err != nil {
		return nil, err
	}
	acquired := false
	if !found {
		ip, err := i.Ipam.AcquireIP(ctx, i.Prefix.Cidr)
		if err != nil {
			return nil, fmt.Errorf("acquire IP: %w", err)
		}
		addr = ip.IP
		if err := writeRecord(recPath, addr); err != nil {
			_ = i.release(ctx, addr)
			return nil, err
		}
		acquired = true
	}

	ipnet := &net.IPNet{IP: net.IP(addr.AsSlice()), Mask: i.ipNet.Mask}

	err = ns.WithNetNSPath(netnsPath, func(ns.NetNS) error {
		link, err := netlink.LinkByName(ifName)
		if err != nil {
			return fmt.Errorf("find interface %q in pod netns: %w", ifName, err)
		}
		if err := netlink.AddrAdd(link, &netlink.Addr{IPNet: ipnet}); err != nil &&
			!errors.Is(err, syscall.EEXIST) {
			return fmt.Errorf("assign %s to %q: %w", ipnet, ifName, err)
		}
		return nil
	})
	if err != nil {
		if acquired { // release what we acquired, not what is on the link
			_ = i.release(ctx, addr)
			_ = os.Remove(recPath)
		}
		return nil, err
	}
	return ipnet, nil
}

// ReleaseIP frees the address of containerID+ifName. Safe to call repeatedly and
// when the netns/link no longer exist (netnsPath may be empty).
func (i *IPAM) ReleaseIP(ctx context.Context, containerID, netnsPath, ifName string) error {
	recPath, err := i.recordPath(containerID, ifName)
	if err != nil {
		return err
	}
	addr, found, err := readRecord(recPath)
	if err != nil {
		return err
	}
	if !found && netnsPath != "" {
		if addr, found, err = i.addrFromLink(netnsPath, ifName); err != nil {
			return err
		}
	}
	if found {
		if err := i.release(ctx, addr); err != nil {
			return err
		}
	}
	if err := os.Remove(recPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove state record: %w", err)
	}
	return nil
}

func (i *IPAM) release(ctx context.Context, addr netip.Addr) error {
	_, err := i.Ipam.ReleaseIP(ctx, &goipam.IP{IP: addr, ParentPrefix: i.Prefix.Cidr})
	if err != nil && !errors.Is(err, goipam.ErrNotFound) {
		return fmt.Errorf("release IP %s: %w", addr, err)
	}
	return nil
}

// addrFromLink is the fallback when no state record exists.
func (i *IPAM) addrFromLink(netnsPath, ifName string) (netip.Addr, bool, error) {
	var out netip.Addr
	found := false
	err := ns.WithNetNSPath(netnsPath, func(ns.NetNS) error {
		link, err := netlink.LinkByName(ifName)
		if err != nil {
			return err
		}
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			return err
		}
		for _, a := range addrs {
			if i.ipNet.Contains(a.IP) && !a.IP.Equal(i.gateway) {
				if v, ok := netip.AddrFromSlice(a.IP.To4()); ok {
					out, found = v, true
					return nil
				}
			}
		}
		return nil
	})
	if err != nil {
		if isGone(err) {
			return netip.Addr{}, false, nil
		}
		return netip.Addr{}, false, fmt.Errorf("read address from %q: %w", ifName, err)
	}
	return out, found, nil
}

func (i *IPAM) recordPath(containerID, ifName string) (string, error) {
	key := containerID + "_" + ifName
	if containerID == "" || ifName == "" || filepath.Base(key) != key || strings.HasPrefix(key, ".") {
		return "", fmt.Errorf("invalid containerID/ifName %q/%q", containerID, ifName)
	}
	return filepath.Join(i.stateDir, key), nil
}

func readRecord(path string) (netip.Addr, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return netip.Addr{}, false, nil
		}
		return netip.Addr{}, false, fmt.Errorf("read state record: %w", err)
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(string(b)))
	if err != nil {
		return netip.Addr{}, false, fmt.Errorf("corrupt state record %s: %w", path, err)
	}
	return addr, true, nil
}

func writeRecord(path string, addr netip.Addr) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create state record: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(addr.String()); err != nil {
		tmp.Close()
		return fmt.Errorf("write state record: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("commit state record: %w", err)
	}
	return nil
}

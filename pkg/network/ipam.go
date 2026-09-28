package network

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/containernetworking/plugins/pkg/ns"
	goipam "github.com/metal-stack/go-ipam"
	"github.com/vishvananda/netlink"
)

type IPAM struct {
	Ipam goipam.Ipamer
	DefaultCIDR string
	Prefix *goipam.Prefix
}

func NewIPAM(ipam goipam.Ipamer, defaultCIDR string) (*IPAM, error) {
	prefix, err := ipam.PrefixFrom(context.Background(), defaultCIDR)
	if err != nil {
		prefix, err = ipam.NewPrefix(context.Background(), defaultCIDR)
		if err != nil {
			return nil, fmt.Errorf("failed to create IP prefix: %v", err)
		}
	}
	return &IPAM{
		Ipam: ipam,
		DefaultCIDR: defaultCIDR,
		Prefix: prefix,
	}, nil
}

func (i *IPAM) AllocateIP(netnsPath string, ifName string, ctx context.Context) error {
	targetNS, err := ns.GetNS(netnsPath)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netnsPath, err)
	}
	defer targetNS.Close()

	ip, err := i.Ipam.AcquireIP(ctx, i.Prefix.Cidr)
	if err != nil {
		return fmt.Errorf("failed to acquire IP from IPAM: %v", err)
	}

	parsedIP := net.ParseIP(ip.IP.String())

	err = targetNS.Do(func(hostNS ns.NetNS) error {
		// Fetch the interface again, now that we are context-shifted inside the pod ns
		containerLink, err := netlink.LinkByName(ifName)
		if err != nil {
			return fmt.Errorf("failed to find interface inside container: %v", err)
		}

		addr := &netlink.Addr{IPNet: &net.IPNet{IP: parsedIP, Mask: net.CIDRMask(16, 32)}}
		err = netlink.AddrAdd(containerLink, addr)
		if err != nil {
			return fmt.Errorf("failed to assign ip address %s: %v", parsedIP, err)
		}
		return nil
	})

	if err != nil {
		i.ReleaseIP(netnsPath, ifName, ctx) // Attempt to release the IP if we failed to assign it
		return fmt.Errorf("failed configuring inside container netns: %v", err)
	}

	return nil
}
func (i *IPAM) ReleaseIP(netnsPath string, ifName string, ctx context.Context) error {
	targetNS, err := ns.GetNS(netnsPath)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netnsPath, err)
	}
	defer targetNS.Close()

	err = targetNS.Do(func(hostNS ns.NetNS) error {
		// Fetch the interface again, now that we are context-shifted inside the pod ns
		containerLink, err := netlink.LinkByName(ifName)
		if err != nil {
			return fmt.Errorf("failed to find interface inside container: %v", err)
		}

		addrs, err := netlink.AddrList(containerLink, netlink.FAMILY_V4)
		if err != nil {
			return fmt.Errorf("Failed to list addresses: %v", err)
		}
		if len(addrs) == 0 {
			return fmt.Errorf("interface %q has no IPv4 addresses", ifName)
		}

		address := addrs[0]
		ip, err := netip.ParseAddr(address.IPNet.IP.String())
		if err != nil {
			return fmt.Errorf("failed to parse address %q: %v", address.IPNet.IP, err)
		}

		if err := netlink.AddrDel(containerLink, &address); err != nil {
			return fmt.Errorf("failed to remove IP address from container link: %v", err)
		}

		if _, err := i.Ipam.ReleaseIP(ctx, &goipam.IP{IP: ip, ParentPrefix: i.DefaultCIDR}); err != nil {
			return fmt.Errorf("failed to release IP %s from IPAM: %v", ip, err)
		}	

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed configuring inside container netns: %v", err)
	}

	return nil
}
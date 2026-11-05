package ebpf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	cebpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/vishvananda/netlink"
)

const (
	PinDirectory  = "/sys/fs/bpf/zero-copy"
	ProgramPath   = PinDirectory + "/xdp_pass"
	LinkDirectory = PinDirectory + "/links"
)

type Agent struct {
	collection *cebpf.Collection
	program    *cebpf.Program
	links      map[int]link.Link
	mu         sync.Mutex
}

// LoadAgent loads the XDP object, reuses the pinned map when present, and
// attaches the program to existing and newly-created host veth interfaces.
func LoadAgent(objectPath string) (*Agent, error) {
	if err := os.MkdirAll(LinkDirectory, 0o755); err != nil {
		return nil, fmt.Errorf("create BPF pin directory: %w", err)
	}
	spec, err := cebpf.LoadCollectionSpec(objectPath)
	if err != nil {
		return nil, fmt.Errorf("load BPF object %q: %w", objectPath, err)
	}

	var pinnedMap *cebpf.Map
	pinnedMap, err = cebpf.LoadPinnedMap(PodInterfaceMapPath, nil)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("open pinned pod map: %w", err)
	}

	pinnedMACMap, macErr := cebpf.LoadPinnedMap(PodMACMapPath, nil)
	if macErr != nil && !os.IsNotExist(macErr) {
		if pinnedMap != nil {
			pinnedMap.Close()
		}
		return nil, fmt.Errorf("open pinned pod MAC map: %w", macErr)
	}
	options := cebpf.CollectionOptions{}
	if pinnedMap != nil {
		options.MapReplacements = map[string]*cebpf.Map{"pod_interface_map": pinnedMap}
	}
	if pinnedMACMap != nil {
		if options.MapReplacements == nil {
			options.MapReplacements = make(map[string]*cebpf.Map)
		}
		options.MapReplacements["pod_mac_map"] = pinnedMACMap
	}
	collection, err := cebpf.NewCollectionWithOptions(spec, options)
	if err != nil {
		if pinnedMap != nil {
			pinnedMap.Close()
		}
		if pinnedMACMap != nil {
			pinnedMACMap.Close()
		}
		return nil, fmt.Errorf("load BPF collection: %w", err)
	}
	program, ok := collection.Programs["xdp_pass"]
	if !ok {
		collection.Close()
		return nil, fmt.Errorf("BPF object does not contain xdp_pass")
	}

	if pinnedMap == nil {
		mapToPin := collection.Maps["pod_interface_map"]
		if mapToPin == nil {
			collection.Close()
			return nil, fmt.Errorf("BPF object does not contain pod_interface_map")
		}
		if err := mapToPin.Pin(PodInterfaceMapPath); err != nil {
			collection.Close()
			return nil, fmt.Errorf("pin pod interface map: %w", err)
		}
	}
	if pinnedMACMap == nil {
		mapToPin := collection.Maps["pod_mac_map"]
		if mapToPin == nil {
			collection.Close()
			return nil, fmt.Errorf("BPF object does not contain pod_mac_map")
		}
		if err := mapToPin.Pin(PodMACMapPath); err != nil {
			collection.Close()
			return nil, fmt.Errorf("pin pod MAC map: %w", err)
		}
	}
	if err := pinProgram(program); err != nil {
		collection.Close()
		return nil, err
	}

	agent := &Agent{collection: collection, program: program, links: make(map[int]link.Link)}
	links, err := netlink.LinkList()
	if err != nil {
		agent.Close()
		return nil, fmt.Errorf("list host links: %w", err)
	}
	for _, candidate := range links {
		if isHostVeth(candidate) {
			if err := agent.Attach(candidate.Attrs().Index); err != nil {
				agent.Close()
				return nil, err
			}
		}
	}
	return agent, nil
}

func pinProgram(program *cebpf.Program) error {
	if _, err := os.Stat(ProgramPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check pinned XDP program: %w", err)
	}
	if err := program.Pin(ProgramPath); err != nil {
		return fmt.Errorf("pin XDP program: %w", err)
	}
	return nil
}

func isHostVeth(candidate netlink.Link) bool {
	return strings.HasPrefix(candidate.Attrs().Name, "veth-")
}

func (a *Agent) Attach(ifindex int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.links[ifindex]; exists {
		return nil
	}
	path := filepath.Join(LinkDirectory, fmt.Sprint(ifindex))
	if pinned, err := link.LoadPinnedLink(path, nil); err == nil {
		a.links[ifindex] = pinned
		return nil
	} else if !os.IsNotExist(err) {
		_ = os.Remove(path)
	}

	xdpLink, err := link.AttachXDP(link.XDPOptions{
		Program:   a.program,
		Interface: ifindex,
		Flags:     link.XDPGenericMode,
	})
	if err != nil {
		return fmt.Errorf("attach XDP to ifindex %d: %w", ifindex, err)
	}
	if err := xdpLink.Pin(path); err != nil {
		xdpLink.Close()
		return fmt.Errorf("pin XDP link for ifindex %d: %w", ifindex, err)
	}
	a.links[ifindex] = xdpLink
	return nil
}

func (a *Agent) Run(ctx context.Context) error {
	updates := make(chan netlink.LinkUpdate)
	done := make(chan struct{})
	defer close(done)
	if err := netlink.LinkSubscribe(updates, done); err != nil {
		return fmt.Errorf("subscribe to link events: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case update := <-updates:
			if update.Link != nil && isHostVeth(update.Link) {
				if err := a.Attach(update.Link.Attrs().Index); err != nil {
					return err
				}
			}
		}
	}
}

func (a *Agent) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for ifindex, xdpLink := range a.links {
		_ = xdpLink.Close()
		delete(a.links, ifindex)
	}
	if a.collection != nil {
		a.collection.Close()
	}
	return nil
}

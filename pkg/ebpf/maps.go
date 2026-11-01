package ebpf

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/cilium/ebpf"
)

const (
	PodInterfaceMapPath = "/sys/fs/bpf/zero-copy/pod_interface_map"
	PodMACMapPath       = "/sys/fs/bpf/zero-copy/pod_mac_map"
)

func openPodInterfaceMap() (*ebpf.Map, error) {
	podMap, err := ebpf.LoadPinnedMap(PodInterfaceMapPath, nil)
	if err != nil {
		return nil, fmt.Errorf("open pinned pod interface map %q: %w", PodInterfaceMapPath, err)
	}
	return podMap, nil
}

func podIPKey(podIPStr string) (uint32, error) {
	ip := net.ParseIP(podIPStr).To4()
	if ip == nil {
		return 0, fmt.Errorf("pod IP %q is not a valid IPv4 address", podIPStr)
	}
	return binary.LittleEndian.Uint32(ip), nil
}

func AddPodToFastPath(podIPStr string, hostVethIfIndex int) error {
	if hostVethIfIndex <= 0 {
		return fmt.Errorf("host veth ifindex must be positive, got %d", hostVethIfIndex)
	}
	ipKey, err := podIPKey(podIPStr)
	if err != nil {
		return err
	}

	podMap, err := openPodInterfaceMap()
	if err != nil {
		return err
	}
	defer podMap.Close()

	val := uint32(hostVethIfIndex)

	return podMap.Put(&ipKey, &val)
}

func DeletePodFromFastPath(podIPStr string) error {
	ipKey, err := podIPKey(podIPStr)
	if err != nil {
		return err
	}

	podMap, err := openPodInterfaceMap()
	if err != nil {
		return err
	}
	defer podMap.Close()

	if err := podMap.Delete(&ipKey); err != nil && err != ebpf.ErrKeyNotExist {
		return fmt.Errorf("delete pod IP %q from fast-path map: %w", podIPStr, err)
	}
	return nil
}

func AddPodMACToFastPath(podIPStr string, mac net.HardwareAddr) error {
	if len(mac) != 6 {
		return fmt.Errorf("pod MAC must contain 6 bytes, got %d", len(mac))
	}
	ipKey, err := podIPKey(podIPStr)
	if err != nil {
		return err
	}
	macMap, err := ebpf.LoadPinnedMap(PodMACMapPath, nil)
	if err != nil {
		return fmt.Errorf("open pinned pod MAC map %q: %w", PodMACMapPath, err)
	}
	defer macMap.Close()
	var value [6]byte
	copy(value[:], mac)
	return macMap.Put(&ipKey, &value)
}

func DeletePodMACFromFastPath(podIPStr string) error {
	ipKey, err := podIPKey(podIPStr)
	if err != nil {
		return err
	}
	macMap, err := ebpf.LoadPinnedMap(PodMACMapPath, nil)
	if err != nil {
		return fmt.Errorf("open pinned pod MAC map %q: %w", PodMACMapPath, err)
	}
	defer macMap.Close()
	if err := macMap.Delete(&ipKey); err != nil && err != ebpf.ErrKeyNotExist {
		return fmt.Errorf("delete pod IP %q from MAC map: %w", podIPStr, err)
	}
	return nil
}

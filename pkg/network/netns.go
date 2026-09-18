package network

import (
	"fmt"
	"runtime"

	"golang.org/x/sys/unix"
)

func MoveToNetNS(path string) error {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open network namespace %q: %w", path, err)
	}
	defer unix.Close(fd)

	if err := unix.Setns(fd, unix.CLONE_NEWNET); err != nil {
		return fmt.Errorf("set network namespace %q: %w", path, err)
	}

	return nil
}

func ConfigureAddress() error {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return fmt.Errorf("create socket: %w", err)
	}
	defer unix.Close(fd)

	ifr, err := unix.NewIfreq("lo")
	if err != nil {
		return fmt.Errorf("create ifreq for lo: %w", err)
	}

	if err := unix.IoctlIfreq(fd, unix.SIOCGIFFLAGS, ifr); err != nil {
		return fmt.Errorf("get lo flags: %w", err)
	}

	flags := ifr.Uint16()
	ifr.SetUint16(flags | unix.IFF_UP)

	if err := unix.IoctlIfreq(fd, unix.SIOCSIFFLAGS, ifr); err != nil {
		return fmt.Errorf("set lo flags: %w", err)
	}

	return nil
}

func ConfigureNetNS(path string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := MoveToNetNS(path); err != nil {
		return err
	}

	return ConfigureAddress()
}

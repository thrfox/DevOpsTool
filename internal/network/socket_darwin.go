package network

import (
	"golang.org/x/sys/unix"
	"syscall"
)

func interfaceControl(index int, name string) func(string, string, syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		var socketErr error
		err := c.Control(func(fd uintptr) { socketErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_BOUND_IF, index) })
		if err != nil {
			return err
		}
		return socketErr
	}
}

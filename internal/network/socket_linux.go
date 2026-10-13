package network

import (
	"golang.org/x/sys/unix"
	"syscall"
)

func interfaceControl(index int, name string) func(string, string, syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		var socketErr error
		err := c.Control(func(fd uintptr) {
			socketErr = unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, name)
		})
		if err != nil {
			return err
		}
		return socketErr
	}
}

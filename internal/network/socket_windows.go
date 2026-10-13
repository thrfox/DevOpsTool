package network

import (
	"golang.org/x/sys/windows"
	"math/bits"
	"syscall"
)

// Windows 的 IP_UNICAST_IF 接收网络字节序的接口索引。
func interfaceControl(index int, name string) func(string, string, syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		var socketErr error
		err := c.Control(func(fd uintptr) {
			socketErr = windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IP, 31, int(bits.ReverseBytes32(uint32(index))))
		})
		if err != nil {
			return err
		}
		return socketErr
	}
}

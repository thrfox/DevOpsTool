package network

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestTunnelNames(t *testing.T) {
	for _, name := range []string{"Mihomo", "utun4", "Wintun", "vEthernet (WSL)", "Tailscale"} {
		if !isTunnelName(name) {
			t.Fatalf("未识别隧道：%s", name)
		}
	}
	if isTunnelName("Ethernet") || isTunnelName("Wi-Fi") {
		t.Fatal("真实网卡被误判")
	}
}

func loopbackOptions(t *testing.T) Options {
	t.Helper()
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, iface := range interfaces {
		addresses, _ := iface.Addrs()
		for _, addr := range addresses {
			ip, _, err := net.ParseCIDR(addr.String())
			if err == nil && ip.String() == "127.0.0.1" {
				return Options{Adapter: Adapter{Index: iface.Index, Name: iface.Name}, Source: "127.0.0.1", Host: "127.0.0.1", Timeout: time.Second}
			}
		}
	}
	t.Skip("当前系统没有 IPv4 回环接口")
	return Options{}
}

func TestBoundTCP(t *testing.T) {
	o := loopbackOptions(t)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
	}()
	o.Port = listener.Addr().(*net.TCPAddr).Port
	result, err := TCP(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "TCP 连接成功") {
		t.Fatal(result)
	}
	o.Source = "192.0.2.123"
	if o.Validate() == nil {
		t.Fatal("应拒绝不属于接口的源地址")
	}
}

func TestDNSCannotUseSystemResolver(t *testing.T) {
	o := loopbackOptions(t)
	o.Host = "example.com"
	o.DNS = ""
	if _, err := Resolve(context.Background(), o); err == nil {
		t.Fatal("域名解析必须指定 DNS，不可悄悄使用系统解析器")
	}
	o.Adapter.Virtual = true
	if o.Validate() == nil {
		t.Fatal("应拒绝虚拟接口")
	}
}

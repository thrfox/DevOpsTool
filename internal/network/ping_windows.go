package network

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/sys/windows"
	"net"
	"strings"
	"time"
	"unsafe"
)

var icmpDLL = windows.NewLazySystemDLL("iphlpapi.dll")
var icmpCreate = icmpDLL.NewProc("IcmpCreateFile")
var icmpClose = icmpDLL.NewProc("IcmpCloseHandle")
var icmpEcho = icmpDLL.NewProc("IcmpSendEcho2Ex")

func pingPlatform(ctx context.Context, o Options, target string, count int) (string, error) {
	if err := checkICMPRoute(ctx, o, target); err != nil {
		return "", err
	}
	h, _, e := icmpCreate.Call()
	if h == ^uintptr(0) {
		return "", e
	}
	defer icmpClose.Call(h)
	var out strings.Builder
	fmt.Fprintf(&out, "目标：%s\n出口：%s / %s\n已核验当前源地址路由\n\n", target, o.Adapter.Name, o.Source)
	received := 0
	for i := 0; i < count; i++ {
		if err := ctx.Err(); err != nil {
			return out.String(), err
		}
		reply := sendEcho(ctx, h, o, target, 64)
		if reply.Status == 0 {
			received++
			fmt.Fprintf(&out, "[%d] 应答：%d ms\n", i+1, reply.Elapsed)
		} else {
			fmt.Fprintf(&out, "[%d] 无应答 / ICMP 状态：%d\n", i+1, reply.Status)
		}
		if i+1 < count {
			select {
			case <-ctx.Done():
				return out.String(), ctx.Err()
			case <-time.After(time.Second):
			}
		}
	}
	fmt.Fprintf(&out, "\n发送 %d，收到 %d，丢包 %.0f%%", count, received, float64(count-received)*100/float64(count))
	return out.String(), nil
}

func checkICMPRoute(ctx context.Context, o Options, target string) error {
	// ICMP API 没有接口绑定选项，因此先核验源地址对应的实际路由；不匹配时拒绝冒充直连。
	data, err := runPS(ctx, `$r=Find-NetRoute -RemoteIPAddress $p.target -LocalIPAddress $p.source | Where-Object { $_.PSObject.Properties.Name -contains 'DestinationPrefix' } | Select-Object -First 1; if (!$r) { throw '找不到源地址对应的路由' }; $r.InterfaceIndex | ConvertTo-Json -Compress`, map[string]any{"target": target, "source": o.Source})
	if err != nil {
		return err
	}
	var index int
	if err := json.Unmarshal(data, &index); err != nil {
		return fmt.Errorf("无法解析出口路由：%s", data)
	}
	if index != o.Adapter.Index {
		return fmt.Errorf("ICMP 当前路由经过接口 %d，所选真实接口为 %d；无法保证绕过 TUN。请调整 VPN 分流或系统路由后重试", index, o.Adapter.Index)
	}
	return nil
}

type icmpResult struct {
	Address string
	Status  uint32
	Elapsed uint32
}
type ipOptions struct {
	TTL, TOS, Flags, OptionsSize uint8
	Data                         uintptr
}

func sendEcho(ctx context.Context, h uintptr, o Options, target string, ttl uint8) icmpResult {
	toNative := func(ip string) uintptr {
		b := net.ParseIP(ip).To4()
		return uintptr(uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24)
	}
	payload := []byte("DevOpsTool diagnostic")
	buffer := make([]byte, 512)
	options := ipOptions{TTL: ttl}
	wait := o.Timeout
	if deadline, ok := ctx.Deadline(); ok {
		wait = min(wait, time.Until(deadline))
	}
	if wait <= 0 {
		return icmpResult{Status: 11010}
	}
	n, _, err := icmpEcho.Call(h, 0, 0, 0, toNative(o.Source), toNative(target), uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), uintptr(unsafe.Pointer(&options)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), uintptr(max(1, wait.Milliseconds())))
	if n == 0 {
		status := uint32(11010)
		if code, ok := err.(windows.Errno); ok {
			status = uint32(code)
		}
		return icmpResult{Status: status}
	}
	return icmpResult{Address: net.IP(buffer[:4]).String(), Status: *(*uint32)(unsafe.Pointer(&buffer[4])), Elapsed: *(*uint32)(unsafe.Pointer(&buffer[8]))}
}

func tracePlatform(ctx context.Context, o Options, target string, maxHops, probes int, progress func(string)) (string, error) {
	if err := checkICMPRoute(ctx, o, target); err != nil {
		return "", err
	}
	h, _, err := icmpCreate.Call()
	if h == ^uintptr(0) {
		return "", err
	}
	defer icmpClose.Call(h)
	var out strings.Builder
	fmt.Fprintf(&out, "Traceroute → %s\n出口：%s / %s\n最大跳数：%d\n\n", target, o.Adapter.Name, o.Source, maxHops)
	for ttl := 1; ttl <= maxHops; ttl++ {
		if err := ctx.Err(); err != nil {
			return out.String(), err
		}
		fmt.Fprintf(&out, "%2d  ", ttl)
		reached := false
		terminal := false
		for probe := 0; probe < probes; probe++ {
			if err := ctx.Err(); err != nil {
				return out.String(), err
			}
			reply := sendEcho(ctx, h, o, target, uint8(ttl))
			switch reply.Status {
			case 0:
				fmt.Fprintf(&out, "%s  %d ms  ", reply.Address, reply.Elapsed)
				reached = reply.Address == target
			case 11013:
				fmt.Fprintf(&out, "%s  %d ms  ", reply.Address, reply.Elapsed)
			case 11010:
				out.WriteString("*  ")
			default:
				fmt.Fprintf(&out, "不可达（%d）  ", reply.Status)
				terminal = true
			}
		}
		out.WriteByte('\n')
		if progress != nil {
			progress(out.String())
		}
		if reached {
			out.WriteString("\n已到达目标。")
			return out.String(), nil
		}
		if terminal {
			return out.String(), fmt.Errorf("路由探测收到不可达响应")
		}
	}
	out.WriteString("\n达到最大跳数，尚未收到目标响应。")
	return out.String(), nil
}

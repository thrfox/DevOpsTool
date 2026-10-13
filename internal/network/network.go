package network

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type Adapter struct {
	Index     int
	Name      string
	ID        string
	Addresses []string
	Virtual   bool
}

func Adapters() ([]Adapter, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var result []Adapter
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		a := Adapter{Index: iface.Index, Name: iface.Name, Virtual: isTunnelName(iface.Name)}
		for _, addr := range addresses {
			ip, _, err := net.ParseCIDR(addr.String())
			if err == nil && ip.To4() != nil && !ip.IsLoopback() && !ip.IsUnspecified() {
				a.Addresses = append(a.Addresses, ip.String())
			}
		}
		if len(a.Addresses) > 0 {
			result = append(result, a)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Virtual != result[j].Virtual {
			return !result[i].Virtual
		}
		return result[i].Index < result[j].Index
	})
	return result, nil
}

func isTunnelName(name string) bool {
	name = strings.ToLower(name)
	for _, marker := range []string{"tun", "tap", "vpn", "wireguard", "tailscale", "clash", "wintun", "utun", "sing-box", "mihomo", "virtual", "vethernet", "docker", "vmware", "zerotier", "loopback"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

type Options struct {
	Adapter    Adapter
	Source     string
	Host       string
	Port       int
	DNS        string
	DNSServers []string
	Timeout    time.Duration
}

func (o Options) Validate() error {
	if o.Timeout < time.Second || o.Timeout > 30*time.Second {
		return fmt.Errorf("超时必须为 1～30 秒")
	}
	if o.Adapter.Index <= 0 || o.Adapter.Virtual {
		return fmt.Errorf("请选择真实网络适配器")
	}
	iface, err := net.InterfaceByIndex(o.Adapter.Index)
	if err != nil || iface.Name != o.Adapter.Name {
		return fmt.Errorf("网络适配器已变化，请刷新列表")
	}
	addresses, err := iface.Addrs()
	if err != nil {
		return err
	}
	found := false
	for _, addr := range addresses {
		ip, _, err := net.ParseCIDR(addr.String())
		if err == nil && ip.String() == o.Source {
			found = true
		}
	}
	if !found || net.ParseIP(o.Source).To4() == nil {
		return fmt.Errorf("源 IPv4 地址不属于所选适配器")
	}
	if strings.TrimSpace(o.Host) == "" || len(o.Host) > 253 || strings.ContainsAny(o.Host, " \t\r\n/\\") || strings.HasPrefix(o.Host, "-") {
		return fmt.Errorf("请输入主机名或 IPv4 地址，不包含协议和路径")
	}
	if o.DNS != "" && net.ParseIP(o.DNS).To4() == nil {
		return fmt.Errorf("DNS 服务器必须为 IPv4 地址")
	}
	if len(o.DNSServers) > 0 {
		if _, err := ParseDNSServers(strings.Join(o.DNSServers, ",")); err != nil {
			return err
		}
	}
	return nil
}

func dialer(o Options, udp bool) *net.Dialer {
	d := &net.Dialer{Timeout: o.Timeout, Control: interfaceControl(o.Adapter.Index, o.Adapter.Name)}
	if udp {
		d.LocalAddr = &net.UDPAddr{IP: net.ParseIP(o.Source)}
	} else {
		d.LocalAddr = &net.TCPAddr{IP: net.ParseIP(o.Source)}
	}
	return d
}

func query(ctx context.Context, o Options, kind uint16) (*dns.Msg, time.Duration, error) {
	servers := o.DNSServers
	if len(servers) == 0 && o.DNS != "" {
		servers = []string{o.DNS}
	}
	if len(servers) == 0 {
		return nil, 0, fmt.Errorf("域名查询必须填写 DNS 服务器，避免使用系统 TUN DNS")
	}
	start := time.Now()
	var lastErr error
	for _, server := range servers {
		if err := ctx.Err(); err != nil {
			return nil, time.Since(start), err
		}
		answer, _, err := queryServer(ctx, o, kind, server)
		if err == nil && answer.Rcode != dns.RcodeServerFailure && answer.Rcode != dns.RcodeRefused {
			return answer, time.Since(start), nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("DNS %s 返回 %s", server, dns.RcodeToString[answer.Rcode])
		}
	}
	return nil, time.Since(start), fmt.Errorf("指定上游 DNS 均未成功应答：%w", lastErr)
}

func queryServer(ctx context.Context, o Options, kind uint16, server string) (*dns.Msg, time.Duration, error) {
	message := new(dns.Msg)
	message.SetQuestion(dns.Fqdn(o.Host), kind)
	client := &dns.Client{Net: "udp4", Timeout: o.Timeout, Dialer: dialer(o, true)}
	answer, elapsed, err := client.ExchangeContext(ctx, message, net.JoinHostPort(server, "53"))
	if err == nil && answer.Truncated {
		client.Net = "tcp4"
		client.Dialer = dialer(o, false)
		return client.ExchangeContext(ctx, message, net.JoinHostPort(server, "53"))
	}
	return answer, elapsed, err
}

func Resolve(ctx context.Context, o Options) (string, error) {
	if err := o.Validate(); err != nil {
		return "", err
	}
	if ip := net.ParseIP(o.Host); ip != nil {
		if ip.To4() == nil {
			return "", fmt.Errorf("初版网络出口调试支持 IPv4")
		}
		return ip.String(), nil
	}
	answer, _, err := query(ctx, o, dns.TypeA)
	if err != nil {
		return "", err
	}
	if answer.Rcode != dns.RcodeSuccess {
		return "", fmt.Errorf("DNS 返回：%s", dns.RcodeToString[answer.Rcode])
	}
	for _, record := range answer.Answer {
		if a, ok := record.(*dns.A); ok {
			return a.A.String(), nil
		}
	}
	return "", fmt.Errorf("DNS 没有返回 IPv4 地址")
}

func TCP(ctx context.Context, o Options) (string, error) {
	if o.Port < 1 || o.Port > 65535 {
		return "", fmt.Errorf("端口必须为 1～65535")
	}
	ip, err := Resolve(ctx, o)
	if err != nil {
		return "", err
	}
	start := time.Now()
	conn, err := dialer(o, false).DialContext(ctx, "tcp4", net.JoinHostPort(ip, strconv.Itoa(o.Port)))
	if err != nil {
		return "", fmt.Errorf("TCP 连接失败（出口 %s / %s）：%w", o.Adapter.Name, o.Source, err)
	}
	defer conn.Close()
	return fmt.Sprintf("TCP 连接成功\n目标：%s:%d\n本地：%s\n出口：%s（索引 %d）\n耗时：%s\n仅检测端口连接，不建立交互式 Telnet 会话。", ip, o.Port, conn.LocalAddr(), o.Adapter.Name, o.Adapter.Index, time.Since(start).Round(time.Microsecond)), nil
}

func DNS(ctx context.Context, o Options, recordType string) (string, error) {
	if err := o.Validate(); err != nil {
		return "", err
	}
	kind, ok := dns.StringToType[recordType]
	if !ok {
		return "", fmt.Errorf("DNS 记录类型无效")
	}
	answer, elapsed, err := query(ctx, o, kind)
	if err != nil {
		return "", fmt.Errorf("DNS 查询失败：%w", err)
	}
	servers := strings.Join(o.DNSServers, ", ")
	if servers == "" {
		servers = o.DNS
	}
	return fmt.Sprintf("出口：%s / %s\n固定上游 DNS：%s\n耗时：%s\n\n%s", o.Adapter.Name, o.Source, servers, elapsed.Round(time.Microsecond), answer.String()), nil
}

func Traceroute(ctx context.Context, o Options, maxHops, probes int, progress func(string)) (string, error) {
	if maxHops < 1 || maxHops > 64 || probes < 1 || probes > 3 {
		return "", fmt.Errorf("最大跳数为 1～64，每跳探测为 1～3 次")
	}
	ip, err := Resolve(ctx, o)
	if err != nil {
		return "", err
	}
	return tracePlatform(ctx, o, ip, maxHops, probes, progress)
}

func Ping(ctx context.Context, o Options, count int) (string, error) {
	if count < 1 || count > 10 {
		return "", fmt.Errorf("Ping 次数必须为 1～10")
	}
	ip, err := Resolve(ctx, o)
	if err != nil {
		return "", err
	}
	return pingPlatform(ctx, o, ip, count)
}

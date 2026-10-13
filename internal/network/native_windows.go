package network

import (
	"context"
	"devopstool/internal/config"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"net"
	"runtime"
	"sort"
	"strings"
	"unsafe"
)

type gatewaySnapshot struct {
	NextHop string
	Metric  uint32
}
type nativeSnapshot struct {
	Index            int
	ID, Name         string
	DHCP, DNSAuto    bool
	DNSMode          string
	DNS              []string
	CurrentAddresses []IPv4Address
	Addresses        []IPv4Address
	Gateways         []gatewaySnapshot
}
type nativeAdapter struct {
	Adapter  Adapter
	Snapshot nativeSnapshot
}

// 原生 API 一次取得地址、网关及 DNS，不启动 PowerShell 或 WMI。
func nativeAdapters(ctx context.Context, onlyIndex ...int) ([]nativeAdapter, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	size := uint32(15 * 1024)
	var buf []byte
	var head *windows.IpAdapterAddresses
	for tries := 0; tries < 3; tries++ {
		if size > 4<<20 {
			return nil, fmt.Errorf("系统网卡信息过大")
		}
		buf = make([]byte, size)
		head = (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
		err := windows.GetAdaptersAddresses(windows.AF_INET, windows.GAA_FLAG_INCLUDE_GATEWAYS|windows.GAA_FLAG_SKIP_ANYCAST|windows.GAA_FLAG_SKIP_MULTICAST, 0, head, &size)
		if errors.Is(err, windows.ERROR_BUFFER_OVERFLOW) {
			continue
		}
		if errors.Is(err, windows.ERROR_NO_DATA) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("原生网卡查询失败：%w", err)
		}
		break
	}
	if head == nil || size > uint32(len(buf)) {
		return nil, fmt.Errorf("网卡列表持续变化，请重试")
	}
	var result []nativeAdapter
	for row := head; row != nil; row = row.Next {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(onlyIndex) > 0 && int(row.IfIndex) != onlyIndex[0] {
			continue
		}
		if row.IfIndex == 0 || row.IfType == windows.IF_TYPE_SOFTWARE_LOOPBACK || row.Flags&0x80 == 0 {
			continue
		}
		id := strings.Trim(windows.BytePtrToString(row.AdapterName), "{}")
		name := windows.UTF16PtrToString(row.FriendlyName)
		info := windows.MibIfRow2{InterfaceIndex: row.IfIndex}
		hardware := windows.GetIfEntry2Ex(windows.MibIfEntryNormalWithoutStatistics, &info) == nil && info.InterfaceAndOperStatusFlags&1 != 0
		a := Adapter{Index: int(row.IfIndex), ID: id, Name: name, Virtual: !hardware || isTunnelName(name)}
		s := nativeSnapshot{Index: a.Index, ID: id, Name: name, DHCP: row.Flags&4 != 0}
		for addr := row.FirstUnicastAddress; addr != nil; addr = addr.Next {
			ip := socketIPv4(addr.Address)
			if ip == "" {
				continue
			}
			origin := "Other"
			if addr.PrefixOrigin == 1 {
				origin = "Manual"
			} else if addr.PrefixOrigin == 3 {
				origin = "Dhcp"
			}
			v := IPv4Address{IP: ip, Prefix: int(addr.OnLinkPrefixLength), Origin: origin}
			s.CurrentAddresses = append(s.CurrentAddresses, v)
			a.Addresses = append(a.Addresses, ip)
			if origin == "Manual" {
				s.Addresses = append(s.Addresses, v)
			}
		}
		for dns := row.FirstDnsServerAddress; dns != nil; dns = dns.Next {
			if ip := socketIPv4(dns.Address); ip != "" {
				s.DNS = append(s.DNS, ip)
			}
		}
		for gw := row.FirstGatewayAddress; gw != nil; gw = gw.Next {
			if ip := socketIPv4(gw.Address); ip != "" && ip != "0.0.0.0" {
				s.Gateways = append(s.Gateways, gatewaySnapshot{NextHop: ip})
			}
		}
		manual, err := manualDNS(id)
		if err != nil {
			return nil, err
		}
		s.DNSAuto = manual == ""
		s.DNSMode = "custom"
		if s.DNSAuto {
			s.DNSMode = "auto"
		}
		if !s.DHCP && len(s.DNS) == 0 {
			s.DNSMode = "none"
			s.DNSAuto = false
		}
		result = append(result, nativeAdapter{Adapter: a, Snapshot: s})
	}
	runtime.KeepAlive(buf)
	sort.Slice(result, func(i, j int) bool { return result[i].Adapter.Index < result[j].Adapter.Index })
	return result, nil
}

func socketIPv4(a windows.SocketAddress) string {
	if a.Sockaddr == nil {
		return ""
	}
	ip := a.IP()
	if ip == nil || ip.To4() == nil {
		return ""
	}
	return ip.String()
}
func manualDNS(id string) (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\{`+id+`}`, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("读取 DNS 配置模式失败：%w", err)
	}
	defer key.Close()
	value, _, err := key.GetStringValue("NameServer")
	if errors.Is(err, registry.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func readNativeSnapshot(ctx context.Context, a Adapter, withRouteMetrics bool) (nativeSnapshot, error) {
	items, err := nativeAdapters(ctx, a.Index)
	if err != nil {
		return nativeSnapshot{}, err
	}
	for _, item := range items {
		if item.Adapter.Index == a.Index {
			if !strings.EqualFold(strings.Trim(item.Adapter.ID, "{}"), strings.Trim(a.ID, "{}")) {
				return nativeSnapshot{}, fmt.Errorf("适配器标识发生变化，请刷新")
			}
			s := item.Snapshot
			if withRouteMetrics {
				gateways, err := nativeDefaultRoutes(a.Index)
				if err != nil {
					return s, err
				}
				s.Gateways = gateways
			}
			return s, nil
		}
	}
	return nativeSnapshot{}, fmt.Errorf("所选 IPv4 适配器不存在，请刷新")
}

func currentFromNative(s nativeSnapshot) CurrentSettings {
	p := config.Profile{Name: s.Name + " 配置", InterfaceName: s.Name, InterfaceID: s.ID, Index: s.Index, DHCP: s.DHCP, Prefix: 24, DNS: s.DNS}
	if len(s.CurrentAddresses) > 0 {
		p.IP = s.CurrentAddresses[0].IP
		p.Prefix = s.CurrentAddresses[0].Prefix
	}
	settings := CurrentSettings{Profile: p, Addresses: s.CurrentAddresses, DNSAuto: s.DNSAuto}
	for _, gw := range s.Gateways {
		settings.Gateways = append(settings.Gateways, gw.NextHop)
	}
	if len(settings.Gateways) > 0 {
		settings.Profile.Gateway = settings.Gateways[0]
	}
	return settings
}

type addressPrefix struct {
	Address [28]byte
	Length  uint8
	Padding [3]byte
}
type forwardRow struct {
	Luid                                               uint64
	Index                                              uint32
	Prefix                                             addressPrefix
	NextHop                                            [28]byte
	SitePrefixLength                                   uint8
	Padding                                            [3]byte
	ValidLifetime, PreferredLifetime, Metric, Protocol uint32
	Loopback, Autoconfigure, Publish, Immortal         uint8
	Age, Origin                                        uint32
}

var getForwardTable = icmpDLL.NewProc("GetIpForwardTable2")
var freeMibTable = icmpDLL.NewProc("FreeMibTable")

func nativeDefaultRoutes(index int) ([]gatewaySnapshot, error) {
	var table *struct {
		Count uint32
		Rows  [1]forwardRow
	}
	r, _, _ := getForwardTable.Call(windows.AF_INET, uintptr(unsafe.Pointer(&table)))
	if r != 0 {
		return nil, fmt.Errorf("读取网关路由失败：%w", windows.Errno(r))
	}
	if table == nil {
		return nil, nil
	}
	defer freeMibTable.Call(uintptr(unsafe.Pointer(table)))
	count := table.Count
	if count > 100000 {
		return nil, fmt.Errorf("路由表过大")
	}
	rows := unsafe.Slice(&table.Rows[0], int(count))
	var gateways []gatewaySnapshot
	for _, row := range rows {
		if int(row.Index) != index || row.Prefix.Length != 0 || binary.LittleEndian.Uint16(row.NextHop[:2]) != windows.AF_INET {
			continue
		}
		ip := net.IP(row.NextHop[4:8])
		if !ip.IsUnspecified() {
			gateways = append(gateways, gatewaySnapshot{NextHop: ip.String(), Metric: row.Metric})
		}
	}
	sort.Slice(gateways, func(i, j int) bool { return gateways[i].Metric < gateways[j].Metric })
	return gateways, nil
}

func marshalNativeSnapshot(s nativeSnapshot) ([]byte, error) { return json.Marshal(s) }

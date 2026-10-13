package network

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestPowerShellScriptParsesWithoutExecution(t *testing.T) {
	// 仅交给 PowerShell 语法分析器检查，绝不执行修改网络的函数。
	script := `[Console]::InputEncoding=[Text.UTF8Encoding]::new(); $tokens=$null; $errors=$null; [System.Management.Automation.Language.Parser]::ParseInput([Console]::In.ReadToEnd(),[ref]$tokens,[ref]$errors) | Out-Null; if ($errors.Count) { $errors | ForEach-Object {$_.Message}; exit 1 }`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Stdin = strings.NewReader(snapshotPS + listAdaptersPS)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("PowerShell 语法错误：%v\n%s", err, out)
	}
}

const mockNetworkCommandsPS = `
$script:events=[Collections.Generic.List[string]]::new();
function Assert-Admin {}
function Get-NetAdapter {param($InterfaceIndex) [PSCustomObject]@{InterfaceGuid=[guid]'11111111-1111-1111-1111-111111111111'}}
function Set-NetIPInterface {param($InterfaceIndex,$AddressFamily,$Dhcp) $script:events.Add('DHCP:'+ $Dhcp)}
function Get-NetRoute {[CmdletBinding()]param($InterfaceIndex,$AddressFamily,$DestinationPrefix) @()}
function Get-NetIPAddress {[CmdletBinding()]param($InterfaceIndex,$AddressFamily) @()}
function Remove-NetRoute {[CmdletBinding(SupportsShouldProcess)]param([Parameter(ValueFromPipeline)]$InputObject)}
function Remove-NetIPAddress {[CmdletBinding(SupportsShouldProcess)]param([Parameter(ValueFromPipeline)]$InputObject)}
function New-NetIPAddress {param($InterfaceIndex,$AddressFamily,$IPAddress,$PrefixLength,$DefaultGateway) if ($p.fail -and $IPAddress -eq '192.0.2.10') {throw '模拟设置失败'}; $script:events.Add('IP:'+ $IPAddress)}
function New-NetRoute {param($InterfaceIndex,$AddressFamily,$DestinationPrefix,$NextHop,$RouteMetric) $script:events.Add('网关:'+ $NextHop)}
function Set-DnsClientServerAddress {param($InterfaceIndex,$ServerAddresses,[switch]$ResetServerAddresses) if($ResetServerAddresses){$script:events.Add('DNS:自动')}else{$script:events.Add('DNS:'+($ServerAddresses -join ','))}}
function netsh.exe { $script:events.Add('DNS:空'); $global:LASTEXITCODE=0 }
`

func TestRestoreWithMockNetwork(t *testing.T) {
	// 所有网络修改命令均被本地模拟函数覆盖，不调用真实网卡操作。
	before := map[string]any{"Index": 2, "ID": "11111111-1111-1111-1111-111111111111", "DHCP": false, "DNSAuto": false, "DNS": []string{"192.0.2.53"}, "Addresses": []any{map[string]any{"IP": "192.0.2.5", "Prefix": 24}}, "Gateways": []any{map[string]any{"NextHop": "192.0.2.1", "Metric": 10}}}
	for _, scenario := range []struct {
		name     string
		dhcp     bool
		fail     bool
		expected []string
	}{{"静态配置恢复", false, false, []string{"IP:192.0.2.5", "DNS:192.0.2.53"}}, {"DHCP 配置恢复", true, false, []string{"DHCP:Enabled", "DNS:自动"}}, {"静态空 DNS 恢复", false, true, []string{"IP:192.0.2.5", "网关:192.0.2.1", "DNS:空"}}} {
		t.Run(scenario.name, func(t *testing.T) {
			b, _ := json.Marshal(before)
			var p map[string]any
			json.Unmarshal(b, &p)
			p["DHCP"] = scenario.dhcp
			p["DNSAuto"] = scenario.dhcp
			if scenario.fail {
				p["DNSMode"] = "none"
				p["DNS"] = []string{}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			script := snapshotPS + mockNetworkCommandsPS + `Restore-Snapshot $p; [PSCustomObject]@{Events=@($script:events);Failure=''} | ConvertTo-Json -Depth 5 -Compress`
			b, err := runPS(ctx, script, p)
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Events  []string
				Failure string
			}
			if err := json.Unmarshal(b, &result); err != nil {
				t.Fatal(err)
			}
			for _, event := range scenario.expected {
				found := false
				for _, actual := range result.Events {
					if event == actual {
						found = true
					}
				}
				if !found {
					t.Fatalf("缺少预期操作 %s：%v", event, result.Events)
				}
			}
			if result.Failure != "" {
				t.Fatal(result.Failure)
			}
		})
	}
}

func TestWindowsReadOnlyNetworkSmoke(t *testing.T) {
	if os.Getenv("DEVOPSTOOL_NETWORK_SMOKE") != "1" {
		t.Skip("按需执行本机只读网卡集成验证")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	adapters, err := ConfigAdapters(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(adapters) == 0 {
		t.Skip("本机没有网卡")
	}
	for _, adapter := range adapters {
		if _, err := CurrentProfile(ctx, adapter); err != nil {
			t.Fatalf("读取适配器 %s 失败：%v", adapter.Name, err)
		}
	}
	physical, err := PhysicalAdapters(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("已读取 %d 个适配器，%d 个物理出口；未修改任何网卡", len(adapters), len(physical))
}

func TestWindowsBoundPingSmoke(t *testing.T) {
	if os.Getenv("DEVOPSTOOL_NETWORK_SMOKE") != "1" {
		t.Skip("按需执行本机出口绑定验证")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	adapters, err := PhysicalAdapters(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(adapters) == 0 {
		t.Skip("没有物理 IPv4 出口")
	}
	a := adapters[0]
	source := a.Addresses[0]
	o := Options{Adapter: a, Source: source, Host: source, Timeout: time.Second}
	result, err := Ping(ctx, o, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "收到 1") {
		t.Fatalf("ICMP 本机地址未应答：%s", result)
	}
	listener, err := net.Listen("tcp4", net.JoinHostPort(source, "0"))
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
	if _, err := TCP(ctx, o); err != nil {
		t.Fatal(err)
	}
	trace, err := Traceroute(ctx, o, 3, 1, nil)
	if err != nil || !strings.Contains(trace, "已到达目标") {
		t.Fatalf("本机 Traceroute 失败：%s %v", trace, err)
	}
	t.Log("已验证物理接口上的本机 ICMP 和 TCP；未向外部主机发送测试流量，未改写网络配置")
}

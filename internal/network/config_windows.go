package network

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"devopstool/internal/config"
)

const listAdaptersPS = `$ipv4=@(Get-NetIPInterface -AddressFamily IPv4 | ForEach-Object {$_.InterfaceIndex}); $items=@(Get-NetAdapter | Where-Object {$ipv4 -contains $_.ifIndex} | Sort-Object ifIndex | ForEach-Object { [PSCustomObject]@{Index=[int]$_.ifIndex;Name=$_.Name;ID=$_.InterfaceGuid.ToString();Virtual=(!$_.HardwareInterface);Addresses=@(Get-NetIPAddress -InterfaceIndex $_.ifIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue | ForEach-Object {$_.IPAddress})} }); ConvertTo-Json -InputObject $items -Depth 5 -Compress`

func ConfigAdapters(ctx context.Context) ([]Adapter, error) {
	items, err := nativeAdapters(ctx)
	if err != nil {
		return nil, err
	}
	adapters := make([]Adapter, len(items))
	for i, item := range items {
		adapters[i] = item.Adapter
	}
	return adapters, nil
}

func PhysicalAdapters(ctx context.Context) ([]Adapter, error) {
	adapters, err := ConfigAdapters(ctx)
	if err != nil {
		return nil, err
	}
	active, err := Adapters()
	if err != nil {
		return nil, err
	}
	var out []Adapter
	for _, a := range active {
		found := false
		for _, details := range adapters {
			if a.Index == details.Index {
				found = true
				a.ID = details.ID
				a.Virtual = a.Virtual || details.Virtual
				break
			}
		}
		if found && !a.Virtual {
			out = append(out, a)
		}
	}
	return out, nil
}

const snapshotPS = `function Get-Snapshot($index) {
 $a=Get-NetAdapter -InterfaceIndex $index;
 $n=Get-NetIPInterface -InterfaceIndex $index -AddressFamily IPv4;
 $key='HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\{'+$a.InterfaceGuid.ToString().Trim('{}')+'}';
 $manual=(Get-ItemProperty -LiteralPath $key -Name NameServer -ErrorAction SilentlyContinue).NameServer;
 $current=@(Get-NetIPAddress -InterfaceIndex $index -AddressFamily IPv4 -ErrorAction SilentlyContinue | Sort-Object @{Expression={if($_.AddressState -eq 'Preferred'){0}else{1}}},IPAddress | ForEach-Object {[PSCustomObject]@{IP=$_.IPAddress;Prefix=[int]$_.PrefixLength;Origin=$_.PrefixOrigin.ToString()}});
 [PSCustomObject]@{Index=[int]$index;ID=$a.InterfaceGuid.ToString();Name=$a.Name;DHCP=($n.Dhcp -eq 'Enabled');DNSAuto=([string]::IsNullOrWhiteSpace($manual));DNS=@((Get-DnsClientServerAddress -InterfaceIndex $index -AddressFamily IPv4).ServerAddresses);CurrentAddresses=$current;Addresses=@($current | Where-Object {$_.Origin -eq 'Manual'});Gateways=@(Get-NetRoute -InterfaceIndex $index -AddressFamily IPv4 -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue | Sort-Object RouteMetric | ForEach-Object {[PSCustomObject]@{NextHop=$_.NextHop;Metric=[int]$_.RouteMetric}})}
}
function Assert-Adapter($index,$id) {
 $a=Get-NetAdapter -InterfaceIndex $index;
 if ([string]::IsNullOrWhiteSpace($id) -or $a.InterfaceGuid.ToString().Trim('{}') -ne $id.Trim('{}')) {throw '适配器标识不匹配，请重新选择适配器'};
}
function Assert-Admin {
 $identity=[Security.Principal.WindowsIdentity]::GetCurrent();
 if (!(New-Object Security.Principal.WindowsPrincipal($identity)).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {throw '更改网络配置需要管理员权限，请以管理员身份重新启动工具'};
}
function Clear-IPv4($index) {
 Set-NetIPInterface -InterfaceIndex $index -AddressFamily IPv4 -Dhcp Disabled;
 Get-NetRoute -InterfaceIndex $index -AddressFamily IPv4 -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue | Remove-NetRoute -Confirm:$false;
 Get-NetIPAddress -InterfaceIndex $index -AddressFamily IPv4 -ErrorAction SilentlyContinue | Remove-NetIPAddress -Confirm:$false;
}
function Restore-Snapshot($s) {
 Assert-Adapter $s.Index $s.ID;
 Clear-IPv4 $s.Index;
 if ($s.DHCP) {Set-NetIPInterface -InterfaceIndex $s.Index -AddressFamily IPv4 -Dhcp Enabled} else {
  foreach ($addr in $s.Addresses) {New-NetIPAddress -InterfaceIndex $s.Index -AddressFamily IPv4 -IPAddress $addr.IP -PrefixLength $addr.Prefix | Out-Null};
  foreach ($gateway in $s.Gateways) {New-NetRoute -InterfaceIndex $s.Index -AddressFamily IPv4 -DestinationPrefix '0.0.0.0/0' -NextHop $gateway.NextHop -RouteMetric $gateway.Metric | Out-Null};
 };
 if ($s.DNSMode -eq 'none') {& netsh.exe interface ipv4 set dnsservers ('name='+$s.Index) source=static address=none validate=no | Out-Null; if($LASTEXITCODE -ne 0){throw '恢复空 DNS 失败'}} elseif ($s.DNSAuto) {Set-DnsClientServerAddress -InterfaceIndex $s.Index -ResetServerAddresses} else {Set-DnsClientServerAddress -InterfaceIndex $s.Index -ServerAddresses @($s.DNS)};
}
`

func CurrentProfile(ctx context.Context, a Adapter) (config.Profile, error) {
	settings, err := CurrentAdapterSettings(ctx, a)
	return settings.Profile, err
}

func CurrentAdapterSettings(ctx context.Context, a Adapter) (CurrentSettings, error) {
	s, err := readNativeSnapshot(ctx, a, true)
	if err != nil {
		return CurrentSettings{}, err
	}
	return currentFromNative(s), nil
}

func settingsFromSnapshot(b []byte, a Adapter) (CurrentSettings, error) {
	var s struct {
		DHCP      bool
		DNSAuto   bool
		DNS       []string
		Addresses []struct {
			IP     string
			Prefix int
		}
		Gateways         []struct{ NextHop string }
		CurrentAddresses []IPv4Address
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return CurrentSettings{}, err
	}
	p := config.Profile{Name: a.Name + " 配置", InterfaceName: a.Name, InterfaceID: a.ID, Index: a.Index, DHCP: s.DHCP, Prefix: 24}
	p.DNS = s.DNS
	addresses := s.CurrentAddresses
	if len(addresses) == 0 {
		for _, addr := range s.Addresses {
			addresses = append(addresses, IPv4Address{IP: addr.IP, Prefix: addr.Prefix, Origin: "Manual"})
		}
	}
	if len(addresses) > 0 {
		p.IP = addresses[0].IP
		p.Prefix = addresses[0].Prefix
	}
	if len(s.Gateways) > 0 {
		p.Gateway = s.Gateways[0].NextHop
	}
	settings := CurrentSettings{Profile: p, Addresses: addresses, DNSAuto: s.DNSAuto}
	for _, gw := range s.Gateways {
		settings.Gateways = append(settings.Gateways, gw.NextHop)
	}
	return settings, nil
}

func backupPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "DevOpsTool", "backups"), nil
}

func ApplyProfile(ctx context.Context, p config.Profile) (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	before, err := readNativeSnapshot(ctx, Adapter{Index: p.Index, ID: p.InterfaceID}, true)
	if err != nil {
		return "", err
	}
	changes := ProfileChanges(p, currentFromNative(before))
	commands := profileCommands(p, changes)
	if len(commands) == 0 {
		return "", nil
	}
	if err := requireAdmin(); err != nil {
		return "", err
	}
	dir, err := backupPath()
	if err != nil {
		return "", err
	}
	return applyCommandsWithBackup(ctx, p, before, commands, dir, runNetCommand, restoreNativeSnapshot)
}

func applyCommandsWithBackup(ctx context.Context, p config.Profile, before nativeSnapshot, commands []netCommand, dir string, run func(context.Context, netCommand) error, restore func(context.Context, nativeSnapshot) error) (string, error) {
	b, err := marshalNativeSnapshot(before)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, time.Now().Format("20060102-150405.000000000")+".json")
	if err := config.WriteAtomic(path, b); err != nil {
		return "", fmt.Errorf("保存备份失败，未修改网络：%w", err)
	}
	if err := config.WriteAtomic(filepath.Join(dir, "last-backup.json"), b); err != nil {
		return "", err
	}
	if err := config.WriteAtomic(filepath.Join(dir, adapterBackupFile(p.InterfaceID)), b); err != nil {
		return "", err
	}
	for _, command := range commands {
		if err := run(ctx, command); err != nil {
			recoveryCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if recoveryErr := restore(recoveryCtx, before); recoveryErr != nil {
				return path, fmt.Errorf("应用失败：%v；恢复也失败：%w\n备份：%s", err, recoveryErr, path)
			}
			return path, fmt.Errorf("应用失败，已恢复原配置：%w\n备份：%s", err, path)
		}
	}
	return path, nil
}

func restoreNativeSnapshot(ctx context.Context, s nativeSnapshot) error {
	_, err := runPS(ctx, snapshotPS+`Assert-Admin; Restore-Snapshot $p`, s)
	return err
}

func RestoreLast(ctx context.Context) error {
	dir, err := backupPath()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(filepath.Join(dir, "last-backup.json"))
	if err != nil {
		return fmt.Errorf("无法读取最近备份：%w", err)
	}
	var before any
	if err := json.Unmarshal(b, &before); err != nil {
		return err
	}
	_, err = runPS(ctx, snapshotPS+`Assert-Admin; Restore-Snapshot $p`, before)
	return err
}

func adapterBackupFile(id string) string {
	return fmt.Sprintf("%x.last.json", sha256.Sum256([]byte(strings.ToLower(strings.Trim(id, "{}")))))
}
func RestoreAdapter(ctx context.Context, a Adapter) error {
	dir, err := backupPath()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(filepath.Join(dir, adapterBackupFile(a.ID)))
	if os.IsNotExist(err) {
		b, err = os.ReadFile(filepath.Join(dir, "last-backup.json"))
	}
	if err != nil {
		return fmt.Errorf("此网卡没有可恢复的备份：%w", err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(b, &snapshot); err != nil {
		return err
	}
	id, _ := snapshot["ID"].(string)
	if !strings.EqualFold(strings.Trim(id, "{}"), strings.Trim(a.ID, "{}")) {
		return fmt.Errorf("备份属于其他适配器，不会修改所选网卡")
	}
	snapshot["Index"] = a.Index
	_, err = runPS(ctx, snapshotPS+`Assert-Admin; Restore-Snapshot $p`, snapshot)
	return err
}

package ui

import (
	"context"
	"devopstool/internal/config"
	"devopstool/internal/network"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"net"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func (d *Desktop) networkConfig() fyne.CanvasObject {
	if runtime.GOOS != "windows" {
		return note("网卡配置仅支持 Windows；其他工具可在当前平台使用。")
	}
	if d.Store == nil {
		return note("本地配置目录不可用")
	}
	adapter := widget.NewSelect(nil, nil)
	saved := widget.NewSelect(nil, nil)
	name := widget.NewEntry()
	name.SetPlaceHolder("例如：设备 A · 192.168.10 网段")
	remark := editor("设备名称、用途或现场备注")
	remark.SetMinRowsVisible(2)
	ip := widget.NewEntry()
	ip.SetPlaceHolder("192.168.10.100")
	mask := widget.NewEntry()
	mask.SetText("255.255.255.0")
	gateway := widget.NewEntry()
	gateway.SetPlaceHolder("直连设备可留空")
	upstream := newDNSChooser(true)
	dnsHint := note("DNS 可留空：DHCP 自动获取 DNS；静态 IP 不配置 DNS。")
	dhcp := widget.NewCheck("自动获取地址（DHCP）", func(value bool) {
		for _, e := range []*widget.Entry{ip, mask, gateway} {
			if value {
				e.Disable()
			} else {
				e.Enable()
			}
		}
	})
	current := note("选择适配器后自动读取其当前设置。")
	status := note("所有方案仅绑定所选网卡；不会修改其他适配器。设备直连可不设置网关。")
	var adapters []network.Adapter
	var profiles []config.Profile
	var editingID string
	var lastOperation string
	cache := make(map[string]network.CurrentSettings)
	cacheTimes := make(map[string]time.Time)
	var formRevision int
	var filling bool
	var allProfiles []config.Profile
	var profilesLoaded bool
	changed := func(string) {
		if !filling {
			formRevision++
		}
	}
	for _, e := range []*widget.Entry{name, remark, ip, mask, gateway, upstream.Addresses} {
		e.OnChanged = changed
	}
	dhcpChanged := dhcp.OnChanged
	dhcp.OnChanged = func(value bool) {
		dhcpChanged(value)
		if !filling {
			formRevision++
		}
	}
	var suppress bool
	var generation int
	var ready bool
	var saveButton, applyButton *widget.Button
	selected := func() network.Adapter {
		for _, a := range adapters {
			if adapterLabel(a) == adapter.Selected {
				return a
			}
		}
		return network.Adapter{}
	}
	setReady := func(value bool) {
		ready = value
		if saveButton != nil && applyButton != nil {
			if value {
				saveButton.Enable()
				applyButton.Enable()
			} else {
				saveButton.Disable()
				applyButton.Disable()
			}
		}
	}
	fill := func(p config.Profile) {
		filling = true
		defer func() { filling = false }()
		editingID = p.ID
		if name.Text != p.Name {
			name.SetText(p.Name)
		}
		if remark.Text != p.Note {
			remark.SetText(p.Note)
		}
		dhcp.SetChecked(p.DHCP)
		if ip.Text != p.IP {
			ip.SetText(p.IP)
		}
		if p.Prefix >= 1 && p.Prefix <= 32 {
			value := net.IP(net.CIDRMask(p.Prefix, 32)).String()
			if mask.Text != value {
				mask.SetText(value)
			}
		}
		if gateway.Text != p.Gateway {
			gateway.SetText(p.Gateway)
		}
		upstream.SetServers(p.DNS)
	}
	fillCurrent := func(s network.CurrentSettings) {
		p := s.Profile
		if s.DNSAuto {
			p.DNS = nil
		}
		fill(p)
	}
	reloadProfiles := func(force ...bool) {
		if !profilesLoaded || (len(force) > 0 && force[0]) {
			items, err := d.Store.List()
			if err != nil {
				dialog.ShowError(err, d.Window)
				return
			}
			allProfiles = items
			profilesLoaded = true
		}
		profiles = network.ProfilesForAdapter(allProfiles, selected())
		labels := make([]string, len(profiles))
		for i, p := range profiles {
			labels[i] = p.Name
		}
		saved.ClearSelected()
		saved.Options = labels
		saved.Refresh()
	}
	var readCurrent func(bool)
	readCurrent = func(fillForm bool) {
		a := selected()
		if a.Index == 0 {
			setReady(false)
			return
		}
		generation++
		request := generation
		revision := formRevision
		key := strings.ToLower(strings.Trim(a.ID, "{}"))
		_, cached := cache[key]
		if !cached {
			setReady(false)
			current.SetText("正在读取 " + a.Name + " 的当前设置…")
		} else {
			status.SetText("已显示缓存（" + cacheTimes[key].Format("15:04:05") + "），正在后台核对…")
		}
		d.pendingTasks++
		go func() {
			started := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			settings, err := d.readAdapterSettings(ctx, a)
			elapsed := time.Since(started)
			fyne.Do(func() {
				defer func() { d.pendingTasks-- }()
				if request != generation || selected().ID != a.ID {
					return
				}
				if err != nil {
					status.SetText("读取失败：" + err.Error())
					if !cached {
						current.SetText("无法读取当前网卡设置，请刷新后重试。")
					} else {
						status.SetText("仍显示缓存，后台核对失败：" + err.Error())
					}
					return
				}
				cache[key] = settings
				cacheTimes[key] = time.Now()
				value := describeCurrent(settings)
				if current.Text != value {
					current.SetText(value)
				}
				if fillForm && revision == formRevision {
					fillCurrent(settings)
				}
				setReady(true)
				message := fmt.Sprintf("已核对 %s · 读取 %.2f ms；仅操作此网卡。", a.Name, float64(elapsed.Microseconds())/1000)
				if lastOperation != "" {
					message = lastOperation + "\n" + message
				}
				status.SetText(message)
			})
		}()
	}
	adapter.OnChanged = func(value string) {
		if suppress {
			return
		}
		editingID = ""
		lastOperation = ""
		generation++
		reloadProfiles()
		a := selected()
		key := strings.ToLower(strings.Trim(a.ID, "{}"))
		if s, ok := cache[key]; ok {
			current.SetText(describeCurrent(s))
			fillCurrent(s)
			setReady(true)
		} else {
			fill(config.Profile{Name: a.Name + " 配置", Prefix: 24})
			setReady(false)
		}
		readCurrent(true)
	}
	saved.OnChanged = func(value string) {
		for _, p := range profiles {
			if p.Name == value {
				formRevision++
				fill(p)
				status.SetText("已加载此网卡方案：" + p.Name + "；当前系统值显示在左侧。")
				return
			}
		}
	}
	getProfile := func() (config.Profile, error) {
		if !ready {
			return config.Profile{}, fmt.Errorf("请等待当前网卡设置读取完成")
		}
		a := selected()
		bits, err := network.Prefix(mask.Text)
		if err != nil && !dhcp.Checked {
			return config.Profile{}, err
		}
		servers, err := upstream.Servers()
		if err != nil {
			return config.Profile{}, err
		}
		p := config.Profile{ID: editingID, Name: strings.TrimSpace(name.Text), Note: remark.Text, Index: a.Index, InterfaceID: a.ID, InterfaceName: a.Name, DHCP: dhcp.Checked, IP: strings.TrimSpace(ip.Text), Prefix: bits, Gateway: strings.TrimSpace(gateway.Text), DNS: servers}
		return p, p.Validate()
	}
	refresh := func() {
		previous := selected().ID
		d.background(func() (func(), error) {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			items, err := d.listConfigAdapters(ctx)
			if err != nil {
				return nil, err
			}
			return func() {
				adapters = items
				labels := make([]string, len(items))
				choice := ""
				for i, a := range items {
					labels[i] = adapterLabel(a)
					if a.ID == previous {
						choice = labels[i]
					}
				}
				if choice == "" {
					for _, a := range items {
						if !a.Virtual {
							choice = adapterLabel(a)
							break
						}
					}
				}
				if choice == "" && len(labels) > 0 {
					choice = labels[0]
				}
				suppress = true
				adapter.ClearSelected()
				adapter.Options = labels
				adapter.Refresh()
				suppress = false
				if choice != "" {
					adapter.SetSelected(choice)
				} else {
					current.SetText("未找到支持 IPv4 的适配器")
					setReady(false)
				}
			}, nil
		}, func(err error) { status.SetText("刷新适配器失败：" + err.Error()) })
	}
	saveButton = widget.NewButton("保存方案", func() {
		p, err := getProfile()
		if err != nil {
			dialog.ShowError(err, d.Window)
			return
		}
		for _, existing := range profiles {
			if existing.Name == p.Name && existing.ID != p.ID {
				dialog.ShowError(fmt.Errorf("此网卡已有同名方案，请更换名称或先加载该方案"), d.Window)
				return
			}
		}
		d.perform(func() (func(), error) {
			p, err := d.Store.Save(p)
			if err != nil {
				return nil, err
			}
			return func() {
				editingID = p.ID
				reloadProfiles(true)
				saved.SetSelected(p.Name)
				status.SetText("已保存到 " + p.InterfaceName + "：" + p.Name)
			}, nil
		})
	})
	applyButton = primary("应用到此网卡", func() {
		p, err := getProfile()
		if err != nil {
			dialog.ShowError(err, d.Window)
			return
		}
		a := selected()
		d.perform(func() (func(), error) {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			settings, err := d.readAdapterSettings(ctx, a)
			if err != nil {
				return nil, err
			}
			return func() {
				dialog.ShowConfirm("确认修改 "+a.Name, describeCurrent(settings)+"\n\n应用方案：\n"+describeProfile(p)+"\n\n仅修改此适配器，操作前保存备份。", func(ok bool) {
					if !ok {
						return
					}
					d.perform(func() (func(), error) {
						started := time.Now()
						ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
						defer cancel()
						path, err := network.ApplyProfile(ctx, p)
						if err != nil {
							return nil, err
						}
						elapsed := time.Since(started)
						return func() {
							key := strings.ToLower(strings.Trim(a.ID, "{}"))
							delete(cache, key)
							result := fmt.Sprintf("设置已提交 · %.2f ms，系统生效可能仍需等待。", float64(elapsed.Microseconds())/1000)
							if path == "" {
								result = "当前设置与方案一致，无需修改。"
							}
							lastOperation = result
							status.SetText(result)
							readCurrent(false)
						}, nil
					})
				}, d.Window)
			}, nil
		})
	})
	newButton := widget.NewButton("另存新方案", func() {
		formRevision++
		editingID = ""
		saved.ClearSelected()
		name.SetText("")
		remark.SetText("")
		status.SetText("保留当前参数，请为此网卡的新方案命名。")
	})
	remove := widget.NewButton("删除", func() {
		id := editingID
		if id == "" {
			return
		}
		dialog.ShowConfirm("删除保存的方案", "仅删除方案记录，不修改适配器。", func(ok bool) {
			if !ok {
				return
			}
			if err := d.Store.Delete(id); err != nil {
				dialog.ShowError(err, d.Window)
				return
			}
			editingID = ""
			reloadProfiles(true)
		}, d.Window)
	})
	restore := widget.NewButton("恢复此网卡备份", func() {
		a := selected()
		if a.Index == 0 {
			return
		}
		dialog.ShowConfirm("恢复 "+a.Name, "只恢复此网卡最近一次应用前的设置。是否继续？", func(ok bool) {
			if !ok {
				return
			}
			d.perform(func() (func(), error) {
				ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
				defer cancel()
				if err := network.RestoreAdapter(ctx, a); err != nil {
					return nil, err
				}
				return func() { readCurrent(true) }, nil
			})
		}, d.Window)
	})
	setReady(false)
	top := container.NewVBox(container.NewBorder(nil, nil, widget.NewLabel("当前适配器"), widget.NewButton("刷新", refresh), adapter), note("插上设备网线 → 选择有线网卡 → 读取当前值 → 切换保存的网段方案"))
	left := container.NewBorder(widget.NewLabelWithStyle("当前系统设置", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), container.NewVBox(widget.NewButton("重新读取当前值", func() { readCurrent(true) }), restore), nil, nil, container.NewVScroll(current))
	form := widget.NewForm(widget.NewFormItem("此网卡的方案", container.NewBorder(nil, nil, nil, remove, saved)), widget.NewFormItem("方案名称", name), widget.NewFormItem("备注", remark), widget.NewFormItem("地址模式", dhcp), widget.NewFormItem("IPv4 地址", ip), widget.NewFormItem("子网掩码 / 前缀", mask), widget.NewFormItem("网关", gateway), widget.NewFormItem("DNS（可留空）", container.NewVBox(upstream.box, dnsHint)))
	right := container.NewBorder(nil, container.NewHBox(newButton, saveButton, applyButton), nil, nil, container.NewVScroll(form))
	split := container.NewHSplit(left, right)
	split.Offset = .38
	page := container.NewBorder(top, status, nil, nil, split)
	refresh()
	return page
}

func describeProfile(p config.Profile) string {
	mode := "静态 IPv4：" + p.IP + "/" + strconv.Itoa(p.Prefix) + "\n网关：" + p.Gateway
	if p.DHCP {
		mode = "IPv4：DHCP 自动获取"
	}
	dns := strings.Join(p.DNS, ", ")
	if dns == "" {
		if p.DHCP {
			dns = "自动获取"
		} else {
			dns = "不配置 DNS"
		}
	}
	return mode + "\nDNS：" + dns
}
func describeCurrent(s network.CurrentSettings) string {
	mode := "手动设置"
	if s.Profile.DHCP {
		mode = "DHCP 自动获取"
	}
	var out strings.Builder
	fmt.Fprintf(&out, "适配器：%s\n地址模式：%s\n\nIPv4 地址 / 子网掩码：\n", s.Profile.InterfaceName, mode)
	if len(s.Addresses) == 0 {
		out.WriteString("尚未分配 IPv4 地址\n")
	}
	for _, addr := range s.Addresses {
		fmt.Fprintf(&out, "%s / %s（/%d）\n", addr.IP, net.IP(net.CIDRMask(addr.Prefix, 32)).String(), addr.Prefix)
	}
	gw := strings.Join(s.Gateways, ", ")
	if gw == "" {
		gw = "未设置"
	}
	servers := strings.Join(s.Profile.DNS, ", ")
	if servers == "" {
		servers = "未配置"
	}
	fmt.Fprintf(&out, "\n默认网关：%s\n\n当前 DNS：%s\n", gw, servers)
	if s.DNSAuto {
		out.WriteString("DNS 来源：自动获取")
	} else {
		out.WriteString("DNS 来源：手动设置")
	}
	return out.String()
}

package ui

import (
	"context"
	"devopstool/internal/network"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"strconv"
	"strings"
	"time"
)

func adapterLabel(a network.Adapter) string { return fmt.Sprintf("%s · #%d", a.Name, a.Index) }

func (d *Desktop) networkPage() fyne.CanvasObject {
	adapter := widget.NewSelect(nil, nil)
	source := widget.NewSelect(nil, nil)
	upstream := newDNSChooser()
	if saved := d.App.Preferences().String("network.upstream"); saved != "" {
		if servers, err := network.ParseDNSServers(saved); err == nil {
			upstream.SetServers(servers)
		}
	}
	var adapters []network.Adapter
	adapter.OnChanged = func(value string) {
		source.ClearSelected()
		source.Options = nil
		for _, a := range adapters {
			if adapterLabel(a) == value {
				source.Options = a.Addresses
				source.Refresh()
				if len(a.Addresses) > 0 {
					source.SetSelected(a.Addresses[0])
				}
				return
			}
		}
		source.Refresh()
	}
	refresh := func() {
		d.perform(func() (func(), error) {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			items, err := d.listPhysicalAdapters(ctx)
			if err != nil {
				return nil, err
			}
			return func() {
				adapters = items
				labels := make([]string, len(items))
				for i, a := range items {
					labels[i] = adapterLabel(a)
				}
				adapter.ClearSelected()
				adapter.Options = labels
				adapter.Refresh()
				if len(labels) > 0 {
					adapter.SetSelected(labels[0])
				}
			}, nil
		})
	}
	getOptions := func(host string, timeout time.Duration) (network.Options, error) {
		var a network.Adapter
		for _, item := range adapters {
			if adapterLabel(item) == adapter.Selected {
				a = item
				break
			}
		}
		servers, err := upstream.Servers()
		if err != nil {
			return network.Options{}, err
		}
		o := network.Options{Adapter: a, Source: source.Selected, Host: strings.TrimSpace(host), Timeout: timeout, DNSServers: servers}
		if err := o.Validate(); err != nil {
			return o, err
		}
		d.App.Preferences().SetString("network.upstream", strings.Join(servers, ","))
		return o, nil
	}
	connection := container.NewGridWithColumns(2, widget.NewForm(widget.NewFormItem("真实出口网卡", adapter), widget.NewFormItem("源 IPv4", source)), widget.NewForm(widget.NewFormItem("固定上游 DNS", upstream.box)))
	top := container.NewVBox(connection, container.NewBorder(nil, nil, nil, widget.NewButton("刷新出口", refresh), note("域名只通过选定上游解析；不会使用系统 DNS。出口绑定失败时显示原因。")), widget.NewSeparator())
	tabs := container.NewAppTabs(container.NewTabItemWithIcon("Ping", moduleIcon("ping"), d.networkTask("Ping", getOptions)), container.NewTabItemWithIcon("Telnet", moduleIcon("terminal"), d.networkTask("Telnet", getOptions)), container.NewTabItemWithIcon("DNS 查询", moduleIcon("dns"), d.networkTask("DNS", getOptions)), container.NewTabItemWithIcon("Traceroute", moduleIcon("route"), d.networkTask("Traceroute", getOptions)))
	page := container.NewBorder(top, nil, nil, nil, tabs)
	refresh()
	return page
}

func (d *Desktop) networkTask(kind string, options func(string, time.Duration) (network.Options, error)) fyne.CanvasObject {
	host := widget.NewEntry()
	host.SetText("example.com")
	timeout := widget.NewSelect([]string{"1", "3", "5", "10", "30"}, nil)
	timeout.SetSelected("3")
	port := widget.NewEntry()
	port.SetText("443")
	count := widget.NewSelect([]string{"1", "4", "10"}, nil)
	count.SetSelected("4")
	record := widget.NewSelect([]string{"A", "AAAA", "CNAME", "MX", "TXT", "NS", "SOA", "PTR"}, nil)
	record.SetSelected("A")
	hops := widget.NewSelect([]string{"8", "16", "30", "64"}, nil)
	hops.SetSelected("30")
	probes := widget.NewSelect([]string{"1", "2", "3"}, nil)
	probes.SetSelected("3")
	output := editor(kind + " 结果将在这里显示")
	form := widget.NewForm(widget.NewFormItem("目标主机 / IP", host), widget.NewFormItem("单次超时（秒）", timeout))
	description := ""
	switch kind {
	case "Ping":
		form.Append("发送次数", count)
		description = "检测 ICMP 延迟与丢包；Windows/macOS 先核验出口路由。"
	case "Telnet":
		form.Append("目标 TCP 端口", port)
		description = "检测端口是否可连接，显示连接耗时；不建立交互式终端。"
	case "DNS":
		form.Append("记录类型", record)
		description = "固定使用上方选择的上游 DNS，通过所选真实接口查询。PTR 查询请填写反向域名。"
	case "Traceroute":
		form.Append("最大跳数", hops)
		form.Append("每跳探测次数", probes)
		description = "逐跳诊断路由；星号表示超时。系统或目标屏蔽 ICMP 时可能无法显示某些跳。"
	}
	var cancel context.CancelFunc
	var start *widget.Button
	stop := widget.NewButton("取消", func() {
		if cancel != nil {
			cancel()
		}
	})
	stop.Disable()
	start = primary("开始 "+kind, func() {
		seconds, _ := strconv.Atoi(timeout.Selected)
		o, err := options(host.Text, time.Duration(seconds)*time.Second)
		if err != nil {
			dialog.ShowError(err, d.Window)
			return
		}
		p, err := strconv.Atoi(strings.TrimSpace(port.Text))
		if kind == "Telnet" && (err != nil || p < 1 || p > 65535) {
			dialog.ShowError(fmt.Errorf("端口必须为 1～65535"), d.Window)
			return
		}
		o.Port = p
		n, _ := strconv.Atoi(count.Selected)
		maxHops, _ := strconv.Atoi(hops.Selected)
		probeCount, _ := strconv.Atoi(probes.Selected)
		recordType := record.Selected
		limit := 45 * time.Second
		if kind == "Traceroute" {
			limit = 180 * time.Second
		}
		ctx, taskCancel := context.WithTimeout(context.Background(), limit)
		cancel = taskCancel
		start.Disable()
		stop.Enable()
		output.SetText("正在通过 " + o.Adapter.Name + " / " + o.Source + " 执行 " + kind + "…")
		go func() {
			defer taskCancel()
			var result string
			var err error
			switch kind {
			case "Ping":
				result, err = network.Ping(ctx, o, n)
			case "Telnet":
				result, err = network.TCP(ctx, o)
			case "DNS":
				result, err = network.DNS(ctx, o, recordType)
			case "Traceroute":
				result, err = network.Traceroute(ctx, o, maxHops, probeCount, func(text string) { fyne.Do(func() { output.SetText(text) }) })
			}
			fyne.Do(func() {
				if err != nil {
					result += "\n\n失败：" + err.Error()
				}
				output.SetText(result)
				start.Enable()
				stop.Disable()
				cancel = nil
			})
		}()
	})
	left := container.NewVBox(form, note(description), container.NewHBox(start, stop))
	return container.NewBorder(nil, d.textActions(output, strings.ToLower(kind)+"-result.txt"), container.NewGridWrap(fyne.NewSize(270, 390), left), nil, output)
}

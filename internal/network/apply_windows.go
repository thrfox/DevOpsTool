package network

import (
	"context"
	"devopstool/internal/config"
	"fmt"
	"golang.org/x/sys/windows"
	"golang.org/x/text/encoding/simplifiedchinese"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"
)

type netCommand struct {
	Args        []string
	Description string
}

func profileCommands(p config.Profile, changes SettingsChanges) []netCommand {
	name := "name=" + strconv.Itoa(p.Index)
	var commands []netCommand
	if changes.Address {
		args := []string{"interface", "ipv4", "set", "address", name}
		if p.DHCP {
			args = append(args, "source=dhcp")
		} else {
			gateway := p.Gateway
			if gateway == "" {
				gateway = "none"
			}
			mask := net.IP(net.CIDRMask(p.Prefix, 32)).String()
			args = append(args, "source=static", "address="+p.IP, "mask="+mask, "gateway="+gateway)
		}
		args = append(args, "store=persistent")
		commands = append(commands, netCommand{Args: args, Description: "设置 IPv4 地址"})
	}
	if changes.DNS {
		args := []string{"interface", "ipv4", "set", "dnsservers", name}
		if len(p.DNS) == 0 {
			if p.DHCP {
				args = append(args, "source=dhcp")
			} else {
				args = append(args, "source=static", "address=none", "validate=no")
			}
		} else {
			args = append(args, "source=static", "address="+p.DNS[0], "validate=no")
		}
		commands = append(commands, netCommand{Args: args, Description: "设置 DNS"})
		if len(p.DNS) > 1 {
			for i, server := range p.DNS[1:] {
				commands = append(commands, netCommand{Args: []string{"interface", "ipv4", "add", "dnsservers", name, "address=" + server, "index=" + strconv.Itoa(i+2), "validate=no"}, Description: "添加备用 DNS"})
			}
		}
	}
	return commands
}

func runNetCommand(ctx context.Context, command netCommand) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "netsh.exe", command.Args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		text := strings.TrimSpace(string(out))
		if !utf8.Valid(out) {
			if decoded, e := simplifiedchinese.GBK.NewDecoder().Bytes(out); e == nil {
				text = strings.TrimSpace(string(decoded))
			}
		}
		return fmt.Errorf("%s失败：%s（%w）", command.Description, text, err)
	}
	return nil
}
func requireAdmin() error {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return fmt.Errorf("更改网卡需要管理员权限，请以管理员身份重新启动工具")
	}
	return nil
}

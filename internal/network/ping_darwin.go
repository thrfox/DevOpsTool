package network

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func pingPlatform(ctx context.Context, o Options, target string, count int) (string, error) {
	// macOS ping 只能指定源地址，必须核验系统路由。
	route, err := exec.CommandContext(ctx, "route", "-n", "get", target).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("路由检查失败：%s", route)
	}
	found := false
	for _, line := range strings.Split(string(route), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "interface:" && fields[1] == o.Adapter.Name {
			found = true
		}
	}
	if !found {
		return "", fmt.Errorf("当前 ICMP 路由未经过所选真实接口，请调整 VPN 分流或系统路由")
	}
	b, err := exec.CommandContext(ctx, "ping", "-n", "-S", o.Source, "-c", strconv.Itoa(count), "-W", strconv.Itoa(int(o.Timeout.Milliseconds())), target).CombinedOutput()
	if err != nil {
		return string(b), fmt.Errorf("Ping 未成功：%w\n%s", err, b)
	}
	return string(b), nil
}

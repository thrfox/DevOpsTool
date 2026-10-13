package network

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func tracePlatform(ctx context.Context, o Options, target string, maxHops, probes int, progress func(string)) (string, error) {
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
		return "", fmt.Errorf("当前路由不经过所选接口，无法确认绕过 TUN")
	}
	b, err := exec.CommandContext(ctx, "traceroute", "-n", "-I", "-i", o.Adapter.Name, "-s", o.Source, "-m", strconv.Itoa(maxHops), "-q", strconv.Itoa(probes), "-w", strconv.Itoa(int(o.Timeout.Seconds())), target).CombinedOutput()
	if err != nil {
		return string(b), fmt.Errorf("Traceroute 失败：%w", err)
	}
	return string(b), nil
}

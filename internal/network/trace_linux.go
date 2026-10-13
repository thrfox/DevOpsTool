package network

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
)

func tracePlatform(ctx context.Context, o Options, target string, maxHops, probes int, progress func(string)) (string, error) {
	b, err := exec.CommandContext(ctx, "traceroute", "-n", "-I", "-i", o.Adapter.Name, "-s", o.Source, "-m", strconv.Itoa(maxHops), "-q", strconv.Itoa(probes), "-w", strconv.Itoa(int(o.Timeout.Seconds())), target).CombinedOutput()
	if err != nil {
		return string(b), fmt.Errorf("Traceroute 失败，请检查 traceroute 是否已安装及接口绑定权限：%w", err)
	}
	return string(b), nil
}

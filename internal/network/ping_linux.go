package network

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
)

func pingPlatform(ctx context.Context, o Options, target string, count int) (string, error) {
	cmd := exec.CommandContext(ctx, "ping", "-n", "-I", o.Adapter.Name, "-c", strconv.Itoa(count), "-W", strconv.Itoa(int(o.Timeout.Seconds())), target)
	b, err := cmd.CombinedOutput()
	text := fmt.Sprintf("出口：%s / %s\n\n%s", o.Adapter.Name, o.Source, b)
	if err != nil {
		return text, fmt.Errorf("Ping 未成功：%w\n%s", err, text)
	}
	return text, nil
}

package network

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"unicode/utf16"
)

// 参数作为 Base64 JSON 传入，防止名称和备注变成 PowerShell 指令。
func runPS(ctx context.Context, script string, args any) ([]byte, error) {
	b, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	script = "$ErrorActionPreference='Stop'; $ProgressPreference='SilentlyContinue'; [Console]::OutputEncoding=[Text.UTF8Encoding]::new(); $p=([Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('" + base64.StdEncoding.EncodeToString(b) + "')) | ConvertFrom-Json); try { " + script + " } catch { [Console]::Error.WriteLine($_.Exception.Message); exit 1 }"
	units := utf16.Encode([]rune(script))
	encoded := make([]byte, len(units)*2)
	for i, v := range units {
		binary.LittleEndian.PutUint16(encoded[i*2:], v)
	}
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(encoded))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("Windows 网络操作超时或已取消：%w", ctx.Err())
		}
		return nil, fmt.Errorf("Windows 网络操作失败：%s（%w）", strings.TrimSpace(string(out)), err)
	}
	return []byte(strings.TrimPrefix(strings.TrimSpace(string(out)), "\ufeff")), nil
}

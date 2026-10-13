//go:build !windows

package network

import (
	"context"
	"devopstool/internal/config"
	"fmt"
)

func ConfigAdapters(ctx context.Context) ([]Adapter, error) { return Adapters() }
func PhysicalAdapters(ctx context.Context) ([]Adapter, error) {
	adapters, err := Adapters()
	if err != nil {
		return nil, err
	}
	var out []Adapter
	for _, a := range adapters {
		if !a.Virtual {
			out = append(out, a)
		}
	}
	return out, nil
}
func CurrentProfile(ctx context.Context, a Adapter) (config.Profile, error) {
	return config.Profile{}, fmt.Errorf("初版网卡配置仅支持 Windows")
}
func CurrentAdapterSettings(ctx context.Context, a Adapter) (CurrentSettings, error) {
	return CurrentSettings{}, fmt.Errorf("网卡设置仅支持 Windows")
}
func ApplyProfile(ctx context.Context, p config.Profile) (string, error) {
	return "", fmt.Errorf("初版网卡配置仅支持 Windows")
}
func RestoreLast(ctx context.Context) error { return fmt.Errorf("初版网卡配置仅支持 Windows") }
func RestoreAdapter(ctx context.Context, a Adapter) error {
	return fmt.Errorf("网卡配置仅支持 Windows")
}

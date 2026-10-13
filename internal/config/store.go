package config

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Profile struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Note          string   `json:"note"`
	InterfaceName string   `json:"interfaceName"`
	InterfaceID   string   `json:"interfaceId"`
	Index         int      `json:"index"`
	DHCP          bool     `json:"dhcp"`
	IP            string   `json:"ip"`
	Prefix        int      `json:"prefix"`
	Gateway       string   `json:"gateway"`
	DNS           []string `json:"dns"`
}

func (p Profile) Validate() error {
	if p.Index <= 0 {
		return fmt.Errorf("请选择网络适配器")
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("请输入配置名称")
	}
	if len(p.Name) > 200 || len(p.Note) > 4000 {
		return fmt.Errorf("名称或备注过长")
	}
	if !p.DHCP {
		ip, err := netip.ParseAddr(p.IP)
		if err != nil || !ip.Is4() || ip.IsUnspecified() || ip.IsMulticast() {
			return fmt.Errorf("请输入有效的 IPv4 地址")
		}
		if p.Prefix < 1 || p.Prefix > 32 {
			return fmt.Errorf("前缀长度必须为 1～32")
		}
		if p.Gateway != "" {
			gw, err := netip.ParseAddr(p.Gateway)
			if err != nil || !gw.Is4() || gw.IsUnspecified() || gw.IsMulticast() {
				return fmt.Errorf("网关必须为有效 IPv4 地址")
			}
			if !netip.PrefixFrom(ip, p.Prefix).Masked().Contains(gw) {
				return fmt.Errorf("网关必须与 IP 位于相同子网")
			}
		}
	}
	if len(p.DNS) > 8 {
		return fmt.Errorf("DNS 服务器最多 8 个")
	}
	for _, value := range p.DNS {
		ip, err := netip.ParseAddr(value)
		if err != nil || !ip.Is4() || ip.IsUnspecified() || ip.IsMulticast() {
			return fmt.Errorf("DNS 必须为有效 IPv4 地址：%s", value)
		}
	}
	return nil
}

type Store struct {
	Path string
	mu   sync.Mutex
}

func NewStore() (*Store, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return &Store{Path: filepath.Join(root, "DevOpsTool", "profiles.json")}, nil
}

func (s *Store) List() ([]Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read()
}

func (s *Store) read() ([]Profile, error) {
	b, err := os.ReadFile(s.Path)
	if os.IsNotExist(err) {
		return []Profile{}, nil
	}
	if err != nil {
		return nil, err
	}
	var profiles []Profile
	if err := json.Unmarshal(b, &profiles); err != nil {
		return nil, fmt.Errorf("配置文件损坏，保留原文件：%w", err)
	}
	return profiles, nil
}

func (s *Store) Save(p Profile) (Profile, error) {
	if err := p.Validate(); err != nil {
		return p, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	profiles, err := s.read()
	if err != nil {
		return p, err
	}
	if p.ID == "" {
		p.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	found := false
	for i := range profiles {
		if profiles[i].ID == p.ID {
			profiles[i] = p
			found = true
			break
		}
	}
	if !found {
		profiles = append(profiles, p)
	}
	return p, s.write(profiles)
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	profiles, err := s.read()
	if err != nil {
		return err
	}
	out := make([]Profile, 0, len(profiles))
	for _, p := range profiles {
		if p.ID != id {
			out = append(out, p)
		}
	}
	return s.write(out)
}

func (s *Store) write(profiles []Profile) error {
	b, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}
	return WriteAtomic(s.Path, b)
}

// 同目录临时文件加重命名，避免程序中断留下半个配置文件。
func WriteAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".devopstool-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

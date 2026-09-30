// Package settings validates and atomically persists monitor connection settings.
package settings

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Connection struct {
	API string `json:"api"`
}

func NormalizeAPI(raw string) (string, error) {
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("请输入有效的 http 或 https API 地址，不含账号、查询参数或片段")
	}
	return strings.TrimRight(u.String(), "/"), nil
}
func Load(path string) (string, error) {
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return "", nil
	}
	if e != nil {
		return "", e
	}
	var c Connection
	if e = json.Unmarshal(b, &c); e != nil {
		return "", e
	}
	return NormalizeAPI(c.API)
}
func Save(path, api string) error {
	if path == "" {
		return fmt.Errorf("未配置设置文件路径")
	}
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".connection-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = json.NewEncoder(f).Encode(Connection{api}); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}

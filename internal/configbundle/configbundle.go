// Package configbundle exposes the configuration that must remain editable
// from LuCI when the smart HTTP panel is unavailable.
package configbundle

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sing-box-smart/internal/config"
	"sing-box-smart/internal/discovery"
	"sing-box-smart/internal/preferences"
	"sing-box-smart/internal/real"
	"sing-box-smart/internal/serviceconfig"
	"sing-box-smart/internal/settings"
	"time"
)

const (
	Dir     = "/etc/sing-box-smart"
	Path    = Dir + "/config.json"
	Pending = Dir + "/config.pending.json"
)

type Bundle struct {
	Connection settings.Connection    `json:"connection"`
	Runtime    preferences.Settings   `json:"runtime"`
	Service    serviceconfig.Settings `json:"service"`
}

func defaults() Bundle {
	c := config.Default()
	return Bundle{
		Connection: settings.Connection{API: c.API, Root: c.Root, Pattern: c.Pattern, AIGroup: c.AIGroup},
		Runtime:    c.Preferences(),
		Service:    serviceconfig.Defaults(),
	}
}

func Load() (Bundle, error) {
	d := defaults()
	connection, err := loadConnectionForEdit(filepath.Join(Dir, "connection.json"), d.Connection)
	if err != nil {
		return Bundle{}, err
	}
	d.Connection = connection
	service, err := loadJSONForEdit(filepath.Join(Dir, "service.json"), d.Service)
	if err != nil {
		return Bundle{}, err
	}
	d.Service = service
	runtime, err := loadJSONForEdit(filepath.Join(Dir, "runtime.json"), d.Runtime)
	if err != nil {
		return Bundle{}, err
	}
	d.Runtime = runtime
	return d, nil
}

// Load is intentionally tolerant for the LuCI recovery path. A syntactically
// valid but semantically invalid value must still be visible and editable when
// the smart process is down.
func loadConnectionForEdit(path string, defaults settings.Connection) (settings.Connection, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaults, nil
	}
	if err != nil {
		return settings.Connection{}, err
	}
	var c = defaults
	if err := json.Unmarshal(b, &c); err != nil {
		return settings.Connection{}, err
	}
	return c, nil
}

func loadJSONForEdit[T any](path string, defaults T) (T, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaults, nil
	}
	if err != nil {
		return defaults, err
	}
	if err := json.Unmarshal(b, &defaults); err != nil {
		return defaults, err
	}
	return defaults, nil
}

func (b Bundle) Validate() error {
	if _, err := settings.NormalizeAPI(b.Connection.API); err != nil {
		return fmt.Errorf("connection: %w", err)
	}
	if _, err := settings.NormalizePattern(b.Connection.Pattern); err != nil {
		return fmt.Errorf("connection: %w", err)
	}
	if b.Connection.Root == "" {
		return fmt.Errorf("connection: 根策略组不能为空")
	}
	if err := b.Runtime.Validate(); err != nil {
		return fmt.Errorf("runtime: %w", err)
	}
	if err := b.Service.Validate(); err != nil {
		return fmt.Errorf("service: %w", err)
	}
	return nil
}

func Save(b Bundle) error {
	if err := b.Validate(); err != nil {
		return err
	}
	if err := settings.Save(filepath.Join(Dir, "connection.json"), b.Connection); err != nil {
		return err
	}
	if err := preferences.Save(filepath.Join(Dir, "runtime.json"), b.Runtime); err != nil {
		return err
	}
	return serviceconfig.Save(filepath.Join(Dir, "service.json"), b.Service)
}

func Command(action string, out io.Writer) error {
	switch action {
	case "read":
		b, err := Load()
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(b)
	case "check":
		b, err := Load()
		if err != nil {
			return err
		}
		if err := b.Validate(); err != nil {
			return err
		}
		cfg := config.Default()
		cfg.Mode = "real"
		cfg.API, cfg.Root, cfg.Pattern = b.Connection.API, b.Connection.Root, b.Connection.Pattern
		cfg.TestURL = b.Runtime.TestURL
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		airports, err := discovery.Load(ctx, real.New(cfg))
		if err != nil {
			return fmt.Errorf("sing-box 配置检查失败: %w", err)
		}
		return json.NewEncoder(out).Encode(map[string]any{"status": "ok", "selectors": len(airports)})
	case "apply":
		f, err := os.Open(Pending)
		if err != nil {
			return err
		}
		defer f.Close()
		var b Bundle
		decoder := json.NewDecoder(io.LimitReader(f, 131073))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&b); err != nil {
			return err
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return fmt.Errorf("设置必须是单个 JSON 对象")
		}
		if err := Save(b); err != nil {
			return err
		}
		_ = os.Remove(Pending)
		return nil
	default:
		return fmt.Errorf("未知配置操作")
	}
}

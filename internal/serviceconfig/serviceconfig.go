// Package serviceconfig handles LuCI's privileged, restart-only settings.
// JSON values are never interpolated into shell commands or sourced as code.
package serviceconfig

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const Path = "/etc/sing-box-smart/service.json"
const Pending = "/etc/sing-box-smart/service.pending.json"

type Settings struct {
	Listen      string `json:"listen"`
	GRPCAddress string `json:"grpc_address"`
	GRPCSecret  string `json:"grpc_secret"`
	LogDir      string `json:"log_dir"`
}

func Defaults() Settings {
	return Settings{Listen: "0.0.0.0:9797", GRPCAddress: "127.0.0.1:9090", LogDir: "/var/log/sing-box-smart"}
}
func validAddress(value string, ipOnly bool) bool {
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		return false
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 || strings.ContainsAny(host, " /\t\r\n") {
		return false
	}
	return host != "" && (!ipOnly || net.ParseIP(host) != nil)
}
func (s Settings) Validate() error {
	if !validAddress(s.Listen, true) {
		return fmt.Errorf("监听地址需为 IP:端口，例如 0.0.0.0:9797")
	}
	host, _, _ := net.SplitHostPort(s.Listen)
	ip := net.ParseIP(host)
	if !ip.IsLoopback() && !ip.IsUnspecified() && !ip.Equal(net.ParseIP("192.168.7.1")) {
		return fmt.Errorf("请使用 0.0.0.0、::、回环地址或 192.168.7.1 监听")
	}
	if s.GRPCAddress != "" && !validAddress(s.GRPCAddress, false) {
		return fmt.Errorf("gRPC 地址需为 主机:端口")
	}
	if len(s.GRPCSecret) > 4096 {
		return fmt.Errorf("gRPC 密钥过长")
	}
	if !filepath.IsAbs(s.LogDir) || filepath.Clean(s.LogDir) == "/" || strings.ContainsAny(s.LogDir, "\x00\r\n") {
		return fmt.Errorf("日志目录必须是绝对目录路径")
	}
	return nil
}
func Load(path string, defaults Settings) (Settings, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaults, nil
	}
	if err != nil {
		return defaults, err
	}
	if err = json.Unmarshal(b, &defaults); err != nil {
		return defaults, err
	}
	return defaults, defaults.Validate()
}
func Save(path string, s Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".service-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = json.NewEncoder(f).Encode(s); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

// Legacy reads only simple assignments. It does not execute a shell file.
func Legacy(path string, s Settings) Settings {
	f, err := os.Open(path)
	if err != nil {
		return s
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		key, value, ok := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"')) {
			value = value[1 : len(value)-1]
		}
		switch key {
		case "LISTEN":
			s.Listen = value
		case "GRPC_ADDR":
			s.GRPCAddress = value
		case "GRPC_SECRET":
			s.GRPCSecret = value
		case "LOG_DIR":
			s.LogDir = value
		}
	}
	return s
}

// Check the effective sleep switch before allowing a transport to be removed.
// Runtime settings take precedence over the legacy startup switch.
func sleepEnabled(path, legacy string) (bool, error) {
	var stored struct {
		Enabled *bool `json:"sleep_enabled"`
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(path), "runtime.json"))
	if err == nil {
		if err = json.Unmarshal(raw, &stored); err != nil {
			return false, err
		}
		if stored.Enabled != nil {
			return *stored.Enabled, nil
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	f, err := os.Open(legacy)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		key, value, ok := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if ok && key == "SLEEP_MONITOR" {
			value = strings.Trim(strings.TrimSpace(value), "\"'")
			return value == "1" || value == "true", nil
		}
	}
	return false, scanner.Err()
}

// Command is used only through fixed LuCI ACL commands. apply consumes one
// staging file and atomically replaces the validated service settings.
func Command(action, path, pending, legacy string, out io.Writer) error {
	switch action {
	case "read":
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		s, err := Load(path, Legacy(legacy, Defaults()))
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(s)
	case "apply":
		f, err := os.Open(pending)
		if err != nil {
			return err
		}
		defer f.Close()
		var s Settings
		decoder := json.NewDecoder(io.LimitReader(f, 16385))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&s); err != nil {
			return err
		}
		var extra any
		if err = decoder.Decode(&extra); err != io.EOF {
			return fmt.Errorf("设置必须是单个 JSON 对象")
		}
		if s.GRPCAddress == "" {
			enabled, err := sleepEnabled(path, legacy)
			if err != nil {
				return err
			}
			if enabled {
				return fmt.Errorf("请先在 9797 关闭自动睡眠，再清空 gRPC 地址")
			}
		}
		if err = Save(path, s); err != nil {
			return err
		}
		_ = os.Remove(pending)
		return nil
	default:
		return fmt.Errorf("未知服务设置操作")
	}
}

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupConfigDir 辅助函数：创建临时目录并写入YAML文件，设置 AGENT_CONFIG_DIR 环境变量，返回清理函数
func setupConfigDir(t *testing.T, files map[string]string) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	origDir, _ := os.LookupEnv("AGENT_CONFIG_DIR")
	origEnv, _ := os.LookupEnv("AGENT_ENV")
	os.Setenv("AGENT_CONFIG_DIR", dir)

	return dir, func() {
		if origDir != "" {
			os.Setenv("AGENT_CONFIG_DIR", origDir)
		} else {
			os.Unsetenv("AGENT_CONFIG_DIR")
		}
		if origEnv != "" {
			os.Setenv("AGENT_ENV", origEnv)
		} else {
			os.Unsetenv("AGENT_ENV")
		}
	}
}

// clearAgentEnvVars 清除所有 AGENT_* 环境变量，防止测试之间互相污染
func clearAgentEnvVars(t *testing.T) {
	t.Helper()
	keys := []string{
		"AGENT_CONFIG_DIR", "AGENT_ENV", "AGENT_CONFIG_FILE",
		"AGENT_LOCAL_ADDR", "AGENT_SERVER_ADDR", "AGENT_REGISTER_TOKEN",
		"AGENT_DEVICE_CODE", "AGENT_VERSION", "AGENT_TENANT_ID",
		"AGENT_DATA_DIR", "AGENT_POLL_TIMEOUT_SECONDS",
		"AGENT_DEFAULT_COMMAND_TIMEOUT_SECONDS", "AGENT_SQLITE_PATH",
		"AGENT_LOG_TO_STDOUT", "AGENT_LOG_FILE_PATH",
		"AGENT_ELK_ENABLED", "AGENT_ELK_ENDPOINT", "AGENT_ELK_INDEX",
		"AGENT_ELK_API_KEY", "AGENT_ELK_TIMEOUT_SECONDS",
		"AGENT_GRAYLOG_ENABLED", "AGENT_GRAYLOG_TRANSPORT",
		"AGENT_GRAYLOG_ENDPOINT", "AGENT_GRAYLOG_HOST",
		"AGENT_GRAYLOG_TIMEOUT_SECONDS", "AGENT_GRAYLOG_LEVEL",
		"AGENT_METRICS_ENABLED", "AGENT_METRICS_PATH",
	}
	for _, k := range keys {
		os.Unsetenv(k)
	}
}

// TestLoadBaseYAML 测试从 base.yaml 加载配置，验证各字段正确解析及超时时间转换
func TestLoadBaseYAML(t *testing.T) {
	clearAgentEnvVars(t)
	baseYAML := `
server_addr: "http://10.0.0.1:8080"
register_token: "test-token"
device_code: "test-device"
poll_timeout_seconds: 60
default_command_timeout_seconds: 120
data_dir: "/tmp/testdata"
`
	_, cleanup := setupConfigDir(t, map[string]string{"base.yaml": baseYAML})
	defer cleanup()
	os.Setenv("AGENT_ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	// 验证字符串字段从 YAML 正确读取
	if cfg.ServerAddr != "http://10.0.0.1:8080" {
		t.Errorf("ServerAddr = %q, want %q", cfg.ServerAddr, "http://10.0.0.1:8080")
	}
	if cfg.RegisterToken != "test-token" {
		t.Errorf("RegisterToken = %q, want %q", cfg.RegisterToken, "test-token")
	}
	if cfg.DeviceCode != "test-device" {
		t.Errorf("DeviceCode = %q, want %q", cfg.DeviceCode, "test-device")
	}
	// 验证秒数被正确转换为 time.Duration
	if cfg.PollTimeout != 60*time.Second {
		t.Errorf("PollTimeout = %v, want %v", cfg.PollTimeout, 60*time.Second)
	}
	if cfg.DefaultTimeout != 120*time.Second {
		t.Errorf("DefaultTimeout = %v, want %v", cfg.DefaultTimeout, 120*time.Second)
	}
}

// TestEnvOverrideDevYAML 测试环境配置文件(dev.yaml)覆盖 base.yaml 中的值
func TestEnvOverrideDevYAML(t *testing.T) {
	clearAgentEnvVars(t)
	baseYAML := `
server_addr: "http://base:8080"
log_to_stdout: false
log_file_path: "base.log"
`
	devYAML := `
server_addr: "http://dev:9090"
log_to_stdout: true
log_file_path: "dev.log"
`
	_, cleanup := setupConfigDir(t, map[string]string{
		"base.yaml": baseYAML,
		"dev.yaml":  devYAML,
	})
	defer cleanup()
	os.Setenv("AGENT_ENV", "dev")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	// 验证 dev.yaml 的值覆盖了 base.yaml
	if cfg.ServerAddr != "http://dev:9090" {
		t.Errorf("ServerAddr = %q, want %q (dev override)", cfg.ServerAddr, "http://dev:9090")
	}
	if !cfg.LogToStdout {
		t.Error("LogToStdout should be true from dev.yaml")
	}
}

// TestEnvVarOverrides 测试 AGENT_* 环境变量覆盖 YAML 配置中的值
func TestEnvVarOverrides(t *testing.T) {
	clearAgentEnvVars(t)
	baseYAML := `
server_addr: "http://base:8080"
device_code: "yaml-device"
poll_timeout_seconds: 10
`
	_, cleanup := setupConfigDir(t, map[string]string{"base.yaml": baseYAML})
	defer cleanup()
	os.Setenv("AGENT_ENV", "")

	// 设置环境变量覆盖
	os.Setenv("AGENT_SERVER_ADDR", "http://env:7070")
	os.Setenv("AGENT_DEVICE_CODE", "env-device")
	os.Setenv("AGENT_POLL_TIMEOUT_SECONDS", "45")
	defer func() {
		os.Unsetenv("AGENT_SERVER_ADDR")
		os.Unsetenv("AGENT_DEVICE_CODE")
		os.Unsetenv("AGENT_POLL_TIMEOUT_SECONDS")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	// 验证环境变量优先级高于 YAML
	if cfg.ServerAddr != "http://env:7070" {
		t.Errorf("ServerAddr = %q, want env override %q", cfg.ServerAddr, "http://env:7070")
	}
	if cfg.DeviceCode != "env-device" {
		t.Errorf("DeviceCode = %q, want env override %q", cfg.DeviceCode, "env-device")
	}
	if cfg.PollTimeout != 45*time.Second {
		t.Errorf("PollTimeout = %v, want %v", cfg.PollTimeout, 45*time.Second)
	}
}

// TestConfigFileNotExist 测试配置文件不存在时（非必需），应回退到默认值而不报错
func TestConfigFileNotExist(t *testing.T) {
	clearAgentEnvVars(t)
	dir := t.TempDir()
	// 空目录，无 base.yaml 和环境 yaml — 两者都是可选的(required=false)
	os.Setenv("AGENT_CONFIG_DIR", dir)
	os.Setenv("AGENT_ENV", "")
	defer func() {
		os.Unsetenv("AGENT_CONFIG_DIR")
		os.Unsetenv("AGENT_ENV")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() should succeed with missing optional files, got: %v", err)
	}
	// 验证回退到默认值
	if cfg.ServerAddr != "http://127.0.0.1:40001" {
		t.Errorf("ServerAddr = %q, want default", cfg.ServerAddr)
	}
}

// TestRequiredConfigFileMissing 测试通过 AGENT_CONFIG_FILE 指定的必需配置文件不存在时应返回错误
func TestRequiredConfigFileMissing(t *testing.T) {
	clearAgentEnvVars(t)
	dir := t.TempDir()
	os.Setenv("AGENT_CONFIG_DIR", dir)
	os.Setenv("AGENT_ENV", "")
	os.Setenv("AGENT_CONFIG_FILE", filepath.Join(dir, "nonexistent.yaml"))
	defer func() {
		os.Unsetenv("AGENT_CONFIG_DIR")
		os.Unsetenv("AGENT_ENV")
		os.Unsetenv("AGENT_CONFIG_FILE")
	}()

	_, err := Load()
	// 必需文件缺失应返回错误
	if err == nil {
		t.Fatal("Load() should fail when required AGENT_CONFIG_FILE is missing")
	}
}

// TestInvalidYAMLFormat 测试无效 YAML 格式应返回解析错误
func TestInvalidYAMLFormat(t *testing.T) {
	clearAgentEnvVars(t)
	invalidYAML := `
server_addr: [invalid yaml
  broken: {{{
`
	_, cleanup := setupConfigDir(t, map[string]string{"base.yaml": invalidYAML})
	defer cleanup()
	os.Setenv("AGENT_ENV", "")

	_, err := Load()
	// 格式错误的 YAML 应导致加载失败
	if err == nil {
		t.Fatal("Load() should fail with invalid YAML")
	}
}

// TestEmptyYAMLFile 测试空白 YAML 文件应正常加载并使用默认值
func TestEmptyYAMLFile(t *testing.T) {
	clearAgentEnvVars(t)
	_, cleanup := setupConfigDir(t, map[string]string{"base.yaml": "   \n\n  "})
	defer cleanup()
	os.Setenv("AGENT_ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() should succeed with empty YAML, got: %v", err)
	}
	// 空文件应回退到默认超时值
	if cfg.PollTimeout != 30*time.Second {
		t.Errorf("PollTimeout = %v, want default 30s", cfg.PollTimeout)
	}
}

// TestNormalize_NegativeTimeouts 测试负数和零值超时被 normalize 修正为默认值 30
func TestNormalize_NegativeTimeouts(t *testing.T) {
	clearAgentEnvVars(t)
	baseYAML := `
poll_timeout_seconds: -5
default_command_timeout_seconds: 0
`
	_, cleanup := setupConfigDir(t, map[string]string{"base.yaml": baseYAML})
	defer cleanup()
	os.Setenv("AGENT_ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	// 负数和零值应被修正为默认值 30
	if cfg.PollTimeoutSeconds != 30 {
		t.Errorf("PollTimeoutSeconds = %d, want 30 (normalized)", cfg.PollTimeoutSeconds)
	}
	if cfg.DefaultTimeoutSeconds != 30 {
		t.Errorf("DefaultTimeoutSeconds = %d, want 30 (normalized)", cfg.DefaultTimeoutSeconds)
	}
}

// TestNormalize_SQLitePathRelative 测试相对路径的 sqlite_path 被拼接到 data_dir 下
func TestNormalize_SQLitePathRelative(t *testing.T) {
	clearAgentEnvVars(t)
	baseYAML := `
data_dir: "/tmp/agentdata"
sqlite_path: "mydb.sqlite"
`
	_, cleanup := setupConfigDir(t, map[string]string{"base.yaml": baseYAML})
	defer cleanup()
	os.Setenv("AGENT_ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	// 相对路径应被拼接为 data_dir + sqlite_path
	want := filepath.Join("/tmp/agentdata", "mydb.sqlite")
	if cfg.SQLitePath != want {
		t.Errorf("SQLitePath = %q, want %q", cfg.SQLitePath, want)
	}
}

// TestNormalize_LogFilePathRelative 测试相对路径的 log_file_path 被拼接到 data_dir 下
func TestNormalize_LogFilePathRelative(t *testing.T) {
	clearAgentEnvVars(t)
	baseYAML := `
data_dir: "/tmp/agentdata"
log_file_path: "app.log"
`
	_, cleanup := setupConfigDir(t, map[string]string{"base.yaml": baseYAML})
	defer cleanup()
	os.Setenv("AGENT_ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	// 相对日志路径应被拼接为 data_dir + log_file_path
	want := filepath.Join("/tmp/agentdata", "app.log")
	if cfg.LogFilePath != want {
		t.Errorf("LogFilePath = %q, want %q", cfg.LogFilePath, want)
	}
}

// TestNormalize_MetricsPathPrefix 测试 metrics_path 缺少前导斜杠时自动补全
func TestNormalize_MetricsPathPrefix(t *testing.T) {
	clearAgentEnvVars(t)
	baseYAML := `metrics_path: "custom-metrics"`
	_, cleanup := setupConfigDir(t, map[string]string{"base.yaml": baseYAML})
	defer cleanup()
	os.Setenv("AGENT_ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	// 缺少 "/" 前缀应自动补全
	if cfg.MetricsPath != "/custom-metrics" {
		t.Errorf("MetricsPath = %q, want %q", cfg.MetricsPath, "/custom-metrics")
	}
}

// TestNormalize_GraylogLevelOutOfRange 测试 graylog_level 超出 0-7 范围时被修正为默认值 6
func TestNormalize_GraylogLevelOutOfRange(t *testing.T) {
	clearAgentEnvVars(t)
	baseYAML := `graylog_level: 99`
	_, cleanup := setupConfigDir(t, map[string]string{"base.yaml": baseYAML})
	defer cleanup()
	os.Setenv("AGENT_ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	// 超出范围的 level 应被修正为默认值 6
	if cfg.GraylogLevel != 6 {
		t.Errorf("GraylogLevel = %d, want 6 (normalized)", cfg.GraylogLevel)
	}
}

// TestNormalize_ForcesLogToStdoutWhenNoOutput 测试当所有日志输出都未配置时，强制启用 stdout 日志
func TestNormalize_ForcesLogToStdoutWhenNoOutput(t *testing.T) {
	clearAgentEnvVars(t)
	baseYAML := `
log_to_stdout: false
log_file_path: ""
elk_enabled: false
graylog_enabled: false
`
	_, cleanup := setupConfigDir(t, map[string]string{"base.yaml": baseYAML})
	defer cleanup()
	os.Setenv("AGENT_ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	// 无任何输出配置时应强制开启 stdout
	if !cfg.LogToStdout {
		t.Error("LogToStdout should be forced true when no output is configured")
	}
}

// TestBoolEnvOverride 测试布尔类型环境变量能正确覆盖 YAML 中的值
func TestBoolEnvOverride(t *testing.T) {
	clearAgentEnvVars(t)
	baseYAML := `elk_enabled: false`
	_, cleanup := setupConfigDir(t, map[string]string{"base.yaml": baseYAML})
	defer cleanup()
	os.Setenv("AGENT_ENV", "")
	os.Setenv("AGENT_ELK_ENABLED", "true")
	defer os.Unsetenv("AGENT_ELK_ENABLED")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	// 环境变量 "true" 应覆盖 YAML 中的 false
	if !cfg.ELKEnabled {
		t.Error("ELKEnabled should be true from env override")
	}
}

// TestInvalidBoolEnvIgnored 测试无效的布尔环境变量值被忽略，保留 YAML 原值
func TestInvalidBoolEnvIgnored(t *testing.T) {
	clearAgentEnvVars(t)
	baseYAML := `elk_enabled: false`
	_, cleanup := setupConfigDir(t, map[string]string{"base.yaml": baseYAML})
	defer cleanup()
	os.Setenv("AGENT_ENV", "")
	os.Setenv("AGENT_ELK_ENABLED", "not-a-bool")
	defer os.Unsetenv("AGENT_ELK_ENABLED")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	// 无效布尔值应被忽略，保持 YAML 中的 false
	if cfg.ELKEnabled {
		t.Error("ELKEnabled should remain false when env value is invalid")
	}
}

// TestInvalidIntEnvIgnored 测试无效的整数环境变量值被忽略，保留 YAML 原值
func TestInvalidIntEnvIgnored(t *testing.T) {
	clearAgentEnvVars(t)
	baseYAML := `poll_timeout_seconds: 20`
	_, cleanup := setupConfigDir(t, map[string]string{"base.yaml": baseYAML})
	defer cleanup()
	os.Setenv("AGENT_ENV", "")
	os.Setenv("AGENT_POLL_TIMEOUT_SECONDS", "abc")
	defer os.Unsetenv("AGENT_POLL_TIMEOUT_SECONDS")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	// 无效整数环境变量被忽略，YAML 值 20 应保留
	if cfg.PollTimeoutSeconds != 20 {
		t.Errorf("PollTimeoutSeconds = %d, want 20 (env invalid, keep yaml)", cfg.PollTimeoutSeconds)
	}
}

// TestReadStringEnv 测试 readStringEnv 辅助函数：环境变量存在时返回其值，不存在时返回 fallback
func TestReadStringEnv(t *testing.T) {
	os.Setenv("TEST_STR_KEY", "hello")
	defer os.Unsetenv("TEST_STR_KEY")

	// 环境变量存在，应返回实际值
	got := readStringEnv("TEST_STR_KEY", "fallback")
	if got != "hello" {
		t.Errorf("readStringEnv = %q, want %q", got, "hello")
	}
	// 环境变量不存在，应返回 fallback
	got = readStringEnv("TEST_STR_MISSING", "fallback")
	if got != "fallback" {
		t.Errorf("readStringEnv = %q, want %q", got, "fallback")
	}
}

// TestReadIntEnv 测试 readIntEnv 辅助函数：正常值、负数、非数字的处理
func TestReadIntEnv(t *testing.T) {
	os.Setenv("TEST_INT_KEY", "42")
	defer os.Unsetenv("TEST_INT_KEY")

	// 正常整数值应正确解析
	got := readIntEnv("TEST_INT_KEY", 10)
	if got != 42 {
		t.Errorf("readIntEnv = %d, want 42", got)
	}

	// 负数应回退到 fallback（代码要求 parsed > 0）
	os.Setenv("TEST_INT_KEY", "-1")
	got = readIntEnv("TEST_INT_KEY", 10)
	if got != 10 {
		t.Errorf("readIntEnv with negative = %d, want fallback 10", got)
	}

	// 非数字字符串应回退到 fallback
	os.Setenv("TEST_INT_KEY", "notanumber")
	got = readIntEnv("TEST_INT_KEY", 10)
	if got != 10 {
		t.Errorf("readIntEnv with invalid = %d, want fallback 10", got)
	}
}

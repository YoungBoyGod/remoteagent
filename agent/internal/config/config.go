package config

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LocalAddr             string        `yaml:"local_addr"`
	ServerAddr            string        `yaml:"server_addr"`
	RegisterToken         string        `yaml:"register_token"`
	DeviceCode            string        `yaml:"device_code"`
	AgentVersion          string        `yaml:"agent_version"`
	TenantID              string        `yaml:"tenant_id"`
	DataDir               string        `yaml:"data_dir"`
	PollTimeoutSeconds    int           `yaml:"poll_timeout_seconds"`
	DefaultTimeoutSeconds int           `yaml:"default_command_timeout_seconds"`
	PollTimeout           time.Duration `yaml:"-"`
	DefaultTimeout        time.Duration `yaml:"-"`
	SQLitePath            string        `yaml:"sqlite_path"`
	LogToStdout           bool          `yaml:"log_to_stdout"`
	LogFilePath           string        `yaml:"log_file_path"`
	ELKEnabled            bool          `yaml:"elk_enabled"`
	ELKEndpoint           string        `yaml:"elk_endpoint"`
	ELKIndex              string        `yaml:"elk_index"`
	ELKAPIKey             string        `yaml:"elk_api_key"`
	ELKTimeoutSeconds     int           `yaml:"elk_timeout_seconds"`
	GraylogEnabled        bool          `yaml:"graylog_enabled"`
	GraylogTransport      string        `yaml:"graylog_transport"`
	GraylogEndpoint       string        `yaml:"graylog_endpoint"`
	GraylogHost           string        `yaml:"graylog_host"`
	GraylogTimeoutSeconds int           `yaml:"graylog_timeout_seconds"`
	GraylogLevel          int           `yaml:"graylog_level"`
	MetricsEnabled        bool          `yaml:"metrics_enabled"`
	MetricsPath           string        `yaml:"metrics_path"`
	MaxConcurrent         int           `yaml:"max_concurrent"`
}

func Load() (Config, error) {
	cfg := defaultConfig()

	configDir := readStringEnv("AGENT_CONFIG_DIR", "./config")
	envName := readStringEnv("AGENT_ENV", "dev")

	if err := mergeYAMLFile(&cfg, filepath.Join(configDir, "base.yaml"), false); err != nil {
		return Config{}, err
	}
	if envName != "" {
		if err := mergeYAMLFile(&cfg, filepath.Join(configDir, envName+".yaml"), false); err != nil {
			return Config{}, err
		}
	}

	customFile := strings.TrimSpace(os.Getenv("AGENT_CONFIG_FILE"))
	if customFile != "" {
		if err := mergeYAMLFile(&cfg, customFile, true); err != nil {
			return Config{}, err
		}
	}

	applyEnvOverrides(&cfg)
	cfg.normalize()
	return cfg, nil
}

// ReloadFrom reloads hot-reloadable fields from environment variables.
// Hot-reloadable: DefaultTimeout, PollTimeout
// Not reloadable: ServerAddr, RegisterToken, DeviceCode, LocalAddr, DataDir
func (c *Config) ReloadFrom() error {
	latest, err := Load()
	if err != nil {
		return err
	}
	c.PollTimeoutSeconds = latest.PollTimeoutSeconds
	c.DefaultTimeoutSeconds = latest.DefaultTimeoutSeconds
	c.PollTimeout = latest.PollTimeout
	c.DefaultTimeout = latest.DefaultTimeout
	return nil
}

func defaultConfig() Config {
	cfg := Config{
		LocalAddr:             "127.0.0.1:40002",
		ServerAddr:            "http://127.0.0.1:40001",
		RegisterToken:         "dev-register-token",
		DeviceCode:            "dev-001",
		AgentVersion:          "0.1.0",
		TenantID:              "default",
		DataDir:               "./data",
		PollTimeoutSeconds:    30,
		DefaultTimeoutSeconds: 30,
		SQLitePath:            "agent.db",
		LogToStdout:           true,
		ELKIndex:              "luoyi-agent",
		ELKTimeoutSeconds:     3,
		GraylogTransport:      "udp",
		GraylogTimeoutSeconds: 3,
		GraylogLevel:          6,
		MetricsEnabled:        true,
		MetricsPath:           "/metrics",
		MaxConcurrent:         4,
	}
	cfg.normalize()
	return cfg
}

func mergeYAMLFile(cfg *Config, path string, required bool) error {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !required {
			return nil
		}
		return err
	}
	if len(strings.TrimSpace(string(content))) == 0 {
		return nil
	}
	return yaml.Unmarshal(content, cfg)
}

func applyEnvOverrides(cfg *Config) {
	if value, ok := readOptionalStringEnv("AGENT_LOCAL_ADDR"); ok {
		cfg.LocalAddr = value
	}
	if value, ok := readOptionalStringEnv("AGENT_SERVER_ADDR"); ok {
		cfg.ServerAddr = value
	}
	if value, ok := readOptionalStringEnv("AGENT_REGISTER_TOKEN"); ok {
		cfg.RegisterToken = value
	}
	if value, ok := readOptionalStringEnv("AGENT_DEVICE_CODE"); ok {
		cfg.DeviceCode = value
	}
	if value, ok := readOptionalStringEnv("AGENT_VERSION"); ok {
		cfg.AgentVersion = value
	}
	if value, ok := readOptionalStringEnv("AGENT_TENANT_ID"); ok {
		cfg.TenantID = value
	}
	if value, ok := readOptionalStringEnv("AGENT_DATA_DIR"); ok {
		cfg.DataDir = value
	}
	if value, ok := readOptionalIntEnv("AGENT_POLL_TIMEOUT_SECONDS"); ok {
		cfg.PollTimeoutSeconds = value
	}
	if value, ok := readOptionalIntEnv("AGENT_DEFAULT_COMMAND_TIMEOUT_SECONDS"); ok {
		cfg.DefaultTimeoutSeconds = value
	}
	if value, ok := readOptionalStringEnv("AGENT_SQLITE_PATH"); ok {
		cfg.SQLitePath = value
	}
	if value, ok := readOptionalBoolEnv("AGENT_LOG_TO_STDOUT"); ok {
		cfg.LogToStdout = value
	}
	if value, ok := readOptionalStringEnv("AGENT_LOG_FILE_PATH"); ok {
		cfg.LogFilePath = value
	}
	if value, ok := readOptionalBoolEnv("AGENT_ELK_ENABLED"); ok {
		cfg.ELKEnabled = value
	}
	if value, ok := readOptionalStringEnv("AGENT_ELK_ENDPOINT"); ok {
		cfg.ELKEndpoint = value
	}
	if value, ok := readOptionalStringEnv("AGENT_ELK_INDEX"); ok {
		cfg.ELKIndex = value
	}
	if value, ok := readOptionalStringEnv("AGENT_ELK_API_KEY"); ok {
		cfg.ELKAPIKey = value
	}
	if value, ok := readOptionalIntEnv("AGENT_ELK_TIMEOUT_SECONDS"); ok {
		cfg.ELKTimeoutSeconds = value
	}
	if value, ok := readOptionalBoolEnv("AGENT_GRAYLOG_ENABLED"); ok {
		cfg.GraylogEnabled = value
	}
	if value, ok := readOptionalStringEnv("AGENT_GRAYLOG_TRANSPORT"); ok {
		cfg.GraylogTransport = value
	}
	if value, ok := readOptionalStringEnv("AGENT_GRAYLOG_ENDPOINT"); ok {
		cfg.GraylogEndpoint = value
	}
	if value, ok := readOptionalStringEnv("AGENT_GRAYLOG_HOST"); ok {
		cfg.GraylogHost = value
	}
	if value, ok := readOptionalIntEnv("AGENT_GRAYLOG_TIMEOUT_SECONDS"); ok {
		cfg.GraylogTimeoutSeconds = value
	}
	if value, ok := readOptionalIntEnv("AGENT_GRAYLOG_LEVEL"); ok {
		cfg.GraylogLevel = value
	}
	if value, ok := readOptionalBoolEnv("AGENT_METRICS_ENABLED"); ok {
		cfg.MetricsEnabled = value
	}
	if value, ok := readOptionalStringEnv("AGENT_METRICS_PATH"); ok {
		cfg.MetricsPath = value
	}
	if value, ok := readOptionalIntEnv("AGENT_MAX_CONCURRENT"); ok {
		cfg.MaxConcurrent = value
	}
}

func (c *Config) normalize() {
	if c.PollTimeoutSeconds <= 0 {
		c.PollTimeoutSeconds = 30
	}
	if c.DefaultTimeoutSeconds <= 0 {
		c.DefaultTimeoutSeconds = 30
	}
	if c.DataDir == "" {
		c.DataDir = "./data"
	}
	if c.SQLitePath == "" {
		c.SQLitePath = "agent.db"
	}
	if c.SQLitePath != "none" && !filepath.IsAbs(c.SQLitePath) {
		c.SQLitePath = filepath.Join(c.DataDir, c.SQLitePath)
	}
	if c.LogFilePath != "" && !filepath.IsAbs(c.LogFilePath) {
		c.LogFilePath = filepath.Join(c.DataDir, c.LogFilePath)
	}
	if c.ELKTimeoutSeconds <= 0 {
		c.ELKTimeoutSeconds = 3
	}
	if c.ELKIndex == "" {
		c.ELKIndex = "luoyi-agent"
	}
	c.GraylogTransport = strings.ToLower(strings.TrimSpace(c.GraylogTransport))
	if c.GraylogTransport == "" {
		c.GraylogTransport = "udp"
	}
	c.GraylogEndpoint = strings.TrimSpace(c.GraylogEndpoint)
	c.GraylogHost = strings.TrimSpace(c.GraylogHost)
	if c.GraylogTimeoutSeconds <= 0 {
		c.GraylogTimeoutSeconds = 3
	}
	if c.GraylogLevel < 0 || c.GraylogLevel > 7 {
		c.GraylogLevel = 6
	}
	if c.MetricsPath == "" {
		c.MetricsPath = "/metrics"
	}
	if !strings.HasPrefix(c.MetricsPath, "/") {
		c.MetricsPath = "/" + c.MetricsPath
	}
	if c.MaxConcurrent <= 0 {
		c.MaxConcurrent = 4
	}
	if !c.LogToStdout && c.LogFilePath == "" && (!c.ELKEnabled || c.ELKEndpoint == "") && (!c.GraylogEnabled || c.GraylogEndpoint == "") {
		c.LogToStdout = true
	}

	c.PollTimeout = time.Duration(c.PollTimeoutSeconds) * time.Second
	c.DefaultTimeout = time.Duration(c.DefaultTimeoutSeconds) * time.Second
}

func readStringEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func readIntEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func readOptionalStringEnv(key string) (string, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(value), true
}

func readOptionalIntEnv(key string) (int, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return 0, false
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}

func readOptionalBoolEnv(key string) (bool, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return false, false
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, false
	}
	return parsed, true
}

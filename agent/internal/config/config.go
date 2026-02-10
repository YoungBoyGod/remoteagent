package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	LocalAddr      string
	ServerAddr     string
	RegisterToken  string
	DeviceCode     string
	AgentVersion   string
	TenantID       string
	DataDir        string
	PollTimeout    time.Duration
	DefaultTimeout time.Duration
}

func Load() Config {
	localAddr := readStringEnv("AGENT_LOCAL_ADDR", "127.0.0.1:40002")
	serverAddr := readStringEnv("AGENT_SERVER_ADDR", "http://127.0.0.1:40001")
	registerToken := readStringEnv("AGENT_REGISTER_TOKEN", "dev-register-token")
	deviceCode := readStringEnv("AGENT_DEVICE_CODE", "dev-001")
	agentVersion := readStringEnv("AGENT_VERSION", "0.1.0")
	tenantID := readStringEnv("AGENT_TENANT_ID", "default")
	dataDir := readStringEnv("AGENT_DATA_DIR", "./data")
	pollTimeoutSeconds := readIntEnv("AGENT_POLL_TIMEOUT_SECONDS", 30)
	defaultTimeoutSeconds := readIntEnv("AGENT_DEFAULT_COMMAND_TIMEOUT_SECONDS", 30)

	return Config{
		LocalAddr:      localAddr,
		ServerAddr:     serverAddr,
		RegisterToken:  registerToken,
		DeviceCode:     deviceCode,
		AgentVersion:   agentVersion,
		TenantID:       tenantID,
		DataDir:        dataDir,
		PollTimeout:    time.Duration(pollTimeoutSeconds) * time.Second,
		DefaultTimeout: time.Duration(defaultTimeoutSeconds) * time.Second,
	}
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

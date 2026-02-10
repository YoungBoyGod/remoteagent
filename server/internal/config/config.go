package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr          string
	RegisterToken string
	JWTTTL        time.Duration
	PollTimeout   time.Duration

	DBHost            string
	DBPort            int
	DBUser            string
	DBPassword        string
	DBName            string
	DBSSLMode         string
	DBConnectTimeoutS int

	LogToStdout           bool
	LogFilePath           string
	GraylogEnabled        bool
	GraylogTransport      string
	GraylogEndpoint       string
	GraylogHost           string
	GraylogTimeoutSeconds int
	GraylogLevel          int
}

func Load() Config {
	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":40001"
	}
	registerToken := os.Getenv("SERVER_REGISTER_TOKEN")
	if registerToken == "" {
		registerToken = "dev-register-token"
	}
	jwtTTLSeconds := readIntEnv("SERVER_JWT_TTL_SECONDS", 86400)
	pollTimeoutSeconds := readIntEnv("SERVER_POLL_TIMEOUT_SECONDS", 30)

	dbHost := readStringEnv("SERVER_DB_HOST", "192.168.10.210")
	dbPort := readIntEnv("SERVER_DB_PORT", 25432)
	dbUser := readStringEnv("SERVER_DB_USER", "remotegpu_user")
	dbPassword := readStringEnv("SERVER_DB_PASSWORD", "remotegpu_password")
	dbName := readStringEnv("SERVER_DB_NAME", "remotegpu")
	dbSSLMode := readStringEnv("SERVER_DB_SSLMODE", "disable")
	dbConnectTimeout := readIntEnv("SERVER_DB_CONNECT_TIMEOUT_SECONDS", 5)

	logToStdout := readBoolEnv("SERVER_LOG_TO_STDOUT", true)
	logFilePath := readStringEnv("SERVER_LOG_FILE_PATH", "")
	graylogEnabled := readBoolEnv("SERVER_GRAYLOG_ENABLED", false)
	graylogTransport := strings.ToLower(strings.TrimSpace(readStringEnv("SERVER_GRAYLOG_TRANSPORT", "udp")))
	graylogEndpoint := strings.TrimSpace(readStringEnv("SERVER_GRAYLOG_ENDPOINT", ""))
	graylogHost := strings.TrimSpace(readStringEnv("SERVER_GRAYLOG_HOST", ""))
	graylogTimeout := readIntEnv("SERVER_GRAYLOG_TIMEOUT_SECONDS", 3)
	graylogLevel := readIntEnv("SERVER_GRAYLOG_LEVEL", 6)
	if graylogLevel > 7 {
		graylogLevel = 6
	}

	return Config{
		Addr:              addr,
		RegisterToken:     registerToken,
		JWTTTL:            time.Duration(jwtTTLSeconds) * time.Second,
		PollTimeout:       time.Duration(pollTimeoutSeconds) * time.Second,
		DBHost:            dbHost,
		DBPort:            dbPort,
		DBUser:            dbUser,
		DBPassword:        dbPassword,
		DBName:            dbName,
		DBSSLMode:             dbSSLMode,
		DBConnectTimeoutS:     dbConnectTimeout,
		LogToStdout:           logToStdout,
		LogFilePath:           logFilePath,
		GraylogEnabled:        graylogEnabled,
		GraylogTransport:      graylogTransport,
		GraylogEndpoint:       graylogEndpoint,
		GraylogHost:           graylogHost,
		GraylogTimeoutSeconds: graylogTimeout,
		GraylogLevel:          graylogLevel,
	}
}

func (c Config) PostgresDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
		c.DBHost,
		c.DBPort,
		quoteIfNeeded(c.DBUser),
		quoteIfNeeded(c.DBPassword),
		quoteIfNeeded(c.DBName),
		quoteIfNeeded(c.DBSSLMode),
		c.DBConnectTimeoutS,
	)
}

func (c Config) PostgresURL() string {
	user := url.UserPassword(c.DBUser, c.DBPassword)
	return fmt.Sprintf("postgres://%s@%s:%d/%s?sslmode=%s", user.String(), c.DBHost, c.DBPort, c.DBName, c.DBSSLMode)
}

// ReloadFrom reloads hot-reloadable fields from environment variables.
// Hot-reloadable: RegisterToken, JWTTTL, PollTimeout
// Not reloadable: Addr (already bound), DB* (connection pool established)
func (c *Config) ReloadFrom() {
	registerToken := os.Getenv("SERVER_REGISTER_TOKEN")
	if registerToken == "" {
		registerToken = "dev-register-token"
	}
	jwtTTLSeconds := readIntEnv("SERVER_JWT_TTL_SECONDS", 86400)
	pollTimeoutSeconds := readIntEnv("SERVER_POLL_TIMEOUT_SECONDS", 30)

	c.RegisterToken = registerToken
	c.JWTTTL = time.Duration(jwtTTLSeconds) * time.Second
	c.PollTimeout = time.Duration(pollTimeoutSeconds) * time.Second
}

func readIntEnv(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func readStringEnv(key string, fallback string) string {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	return raw
}

func readBoolEnv(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return parsed
}

func quoteIfNeeded(raw string) string {
	if raw == "" {
		return "''"
	}
	return raw
}

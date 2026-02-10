package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
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
		DBSSLMode:         dbSSLMode,
		DBConnectTimeoutS: dbConnectTimeout,
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

func quoteIfNeeded(raw string) string {
	if raw == "" {
		return "''"
	}
	return raw
}

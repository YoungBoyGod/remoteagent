package config

import (
	"fmt"
	"log"
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

	RedisAddr     string // 环境变量 REDIS_ADDR，默认 "localhost:26379"
	RedisPassword string // 环境变量 REDIS_PASSWORD，默认 ""
	RedisDB       int    // 环境变量 REDIS_DB，默认 0

	LogToStdout           bool
	LogFilePath           string
	GraylogEnabled        bool
	GraylogTransport      string
	GraylogEndpoint       string
	GraylogHost           string
	GraylogTimeoutSeconds int
	GraylogLevel          int

	S3Endpoint        string // 环境变量 S3_ENDPOINT，S3/MinIO/RustFS 端点地址
	S3Region          string // 环境变量 S3_REGION，默认 "us-east-1"
	S3Bucket          string // 环境变量 S3_BUCKET，默认 "doccenter"
	S3AccessKeyID     string // 环境变量 S3_ACCESS_KEY_ID
	S3SecretAccessKey string // 环境变量 S3_SECRET_ACCESS_KEY
	S3UsePathStyle    bool   // 环境变量 S3_USE_PATH_STYLE，MinIO/RustFS 需要 true，默认 true

	EnableSwagger bool // 环境变量 SERVER_ENABLE_SWAGGER，生产环境应设为 false，默认 true
}

func Load() Config {
	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":40001"
	}
	registerToken := os.Getenv("SERVER_REGISTER_TOKEN")
	jwtTTLSeconds := readIntEnv("SERVER_JWT_TTL_SECONDS", 86400)
	pollTimeoutSeconds := readIntEnv("SERVER_POLL_TIMEOUT_SECONDS", 30)

	dbHost := readStringEnv("SERVER_DB_HOST", "localhost")
	dbPort := readIntEnv("SERVER_DB_PORT", 25432)
	dbUser := readStringEnv("SERVER_DB_USER", "remotegpu_user")
	dbPassword := readStringEnv("SERVER_DB_PASSWORD", "")
	dbName := readStringEnv("SERVER_DB_NAME", "remotegpu")
	dbSSLMode := readStringEnv("SERVER_DB_SSLMODE", "disable")
	dbConnectTimeout := readIntEnv("SERVER_DB_CONNECT_TIMEOUT_SECONDS", 5)

	redisAddr := readStringEnv("REDIS_ADDR", "localhost:26379")
	redisPassword := readStringEnv("REDIS_PASSWORD", "")
	redisDB := readIntEnvAllowZero("REDIS_DB", 0)

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

	s3Endpoint := readStringEnv("S3_ENDPOINT", "http://localhost:29000")
	s3Region := readStringEnv("S3_REGION", "us-east-1")
	s3Bucket := readStringEnv("S3_BUCKET", "doccenter")
	s3AccessKeyID := readStringEnv("S3_ACCESS_KEY_ID", "")
	if s3AccessKeyID == "" {
		s3AccessKeyID = readStringEnv("S3_ACCESS_KEY", "")
	}
	s3SecretAccessKey := readStringEnv("S3_SECRET_ACCESS_KEY", "")
	if s3SecretAccessKey == "" {
		s3SecretAccessKey = readStringEnv("S3_SECRET_KEY", "")
	}
	s3UsePathStyle := readBoolEnv("S3_USE_PATH_STYLE", true)

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
		RedisAddr:             redisAddr,
		RedisPassword:         redisPassword,
		RedisDB:               redisDB,
		LogToStdout:           logToStdout,
		LogFilePath:           logFilePath,
		GraylogEnabled:        graylogEnabled,
		GraylogTransport:      graylogTransport,
		GraylogEndpoint:       graylogEndpoint,
		GraylogHost:           graylogHost,
		GraylogTimeoutSeconds: graylogTimeout,
		GraylogLevel:          graylogLevel,
		S3Endpoint:            s3Endpoint,
		S3Region:              s3Region,
		S3Bucket:              s3Bucket,
		S3AccessKeyID:         s3AccessKeyID,
		S3SecretAccessKey:     s3SecretAccessKey,
		S3UsePathStyle:        s3UsePathStyle,
		EnableSwagger:         readBoolEnv("SERVER_ENABLE_SWAGGER", true),
	}
}

func (c Config) Validate() {
	if c.RegisterToken == "" {
		log.Fatal("SERVER_REGISTER_TOKEN is required")
	}
	if c.DBPassword == "" {
		log.Fatal("SERVER_DB_PASSWORD is required")
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
	jwtTTLSeconds := readIntEnv("SERVER_JWT_TTL_SECONDS", 86400)
	pollTimeoutSeconds := readIntEnv("SERVER_POLL_TIMEOUT_SECONDS", 30)

	if registerToken != "" {
		c.RegisterToken = registerToken
	}
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

func readIntEnvAllowZero(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
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

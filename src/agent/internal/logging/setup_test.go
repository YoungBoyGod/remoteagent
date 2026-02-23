package logging

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"luoyi2026/agent/internal/config"
)

// TestSetup_StdoutOnly 测试仅配置 stdout 输出时，Setup 正常初始化且日志写入不 panic
func TestSetup_StdoutOnly(t *testing.T) {
	cfg := config.Config{
		LogToStdout: true,
	}
	cleanup, err := Setup(cfg)
	if err != nil {
		t.Fatalf("Setup() error: %v", err)
	}
	defer cleanup()

	// 验证日志写入 stdout 不会 panic
	log.Println("test stdout logging")
}

// TestSetup_FileLogging 测试文件日志输出：日志内容应写入指定文件
func TestSetup_FileLogging(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "subdir", "test.log")

	cfg := config.Config{
		LogToStdout: false,
		LogFilePath: logPath,
	}
	cleanup, err := Setup(cfg)
	if err != nil {
		t.Fatalf("Setup() error: %v", err)
	}

	log.Println("file log entry")
	cleanup()

	// 验证日志文件中包含写入的内容
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if !strings.Contains(string(content), "file log entry") {
		t.Errorf("log file should contain 'file log entry', got: %s", string(content))
	}
}

// TestSetup_FileLoggingCreatesDir 测试 Setup 自动创建日志文件的父目录（多层嵌套）
func TestSetup_FileLoggingCreatesDir(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "deep", "nested", "test.log")

	cfg := config.Config{
		LogToStdout: false,
		LogFilePath: logPath,
	}
	cleanup, err := Setup(cfg)
	if err != nil {
		t.Fatalf("Setup() error: %v", err)
	}
	defer cleanup()

	// 验证父目录已被自动创建
	if _, err := os.Stat(filepath.Dir(logPath)); os.IsNotExist(err) {
		t.Error("Setup should create parent directories for log file")
	}
}

// TestSetup_InvalidLogFilePath 测试无效的日志文件路径应返回错误
func TestSetup_InvalidLogFilePath(t *testing.T) {
	cfg := config.Config{
		LogToStdout: false,
		LogFilePath: "/dev/null/impossible/path/test.log",
	}
	_, err := Setup(cfg)
	// 不可能的路径应导致 Setup 失败
	if err == nil {
		t.Fatal("Setup() should fail with invalid log file path")
	}
}

// TestSetup_FallbackToStdout 测试当未配置任何 writer 时，自动回退到 stdout 输出
func TestSetup_FallbackToStdout(t *testing.T) {
	cfg := config.Config{
		LogToStdout: false,
		LogFilePath: "",
	}
	cleanup, err := Setup(cfg)
	if err != nil {
		t.Fatalf("Setup() error: %v", err)
	}
	defer cleanup()

	// 无 writer 时应回退到 stdout，写入不 panic
	log.Println("fallback stdout test")
}

// TestSetup_GraylogConfigParsing 测试 Graylog HTTP 传输模式的配置解析和初始化
func TestSetup_GraylogConfigParsing(t *testing.T) {
	cfg := config.Config{
		LogToStdout:           true,
		GraylogEnabled:        true,
		GraylogTransport:      "http",
		GraylogEndpoint:       "http://127.0.0.1:12201/gelf",
		GraylogHost:           "test-host",
		GraylogTimeoutSeconds: 5,
		GraylogLevel:          4,
	}
	cleanup, err := Setup(cfg)
	if err != nil {
		t.Fatalf("Setup() with HTTP graylog error: %v", err)
	}
	defer cleanup()

	// 验证 HTTP 模式的 Graylog writer 正常工作
	log.Println("graylog http test")
}

// TestSetup_GraylogDisabledEmptyEndpoint 测试 Graylog 启用但 endpoint 为空白时应跳过初始化
func TestSetup_GraylogDisabledEmptyEndpoint(t *testing.T) {
	cfg := config.Config{
		LogToStdout:      true,
		GraylogEnabled:   true,
		GraylogTransport: "udp",
		GraylogEndpoint:  "   ",
	}
	cleanup, err := Setup(cfg)
	// endpoint 为空白字符串时应跳过 Graylog，不报错
	if err != nil {
		t.Fatalf("Setup() should skip graylog with empty endpoint, got: %v", err)
	}
	defer cleanup()
}

// TestSetup_ELKDisabledEmptyEndpoint 测试 ELK 启用但 endpoint 为空白时应跳过初始化
func TestSetup_ELKDisabledEmptyEndpoint(t *testing.T) {
	cfg := config.Config{
		LogToStdout: true,
		ELKEnabled:  true,
		ELKEndpoint: "  ",
	}
	cleanup, err := Setup(cfg)
	// endpoint 为空白字符串时应跳过 ELK，不报错
	if err != nil {
		t.Fatalf("Setup() should skip ELK with empty endpoint, got: %v", err)
	}
	defer cleanup()
}

// TestSetup_ELKEnabled 测试 ELK 完整配置（endpoint、index、apiKey）时正常初始化
func TestSetup_ELKEnabled(t *testing.T) {
	cfg := config.Config{
		LogToStdout:       true,
		ELKEnabled:        true,
		ELKEndpoint:       "http://127.0.0.1:9200",
		ELKIndex:          "test-index",
		ELKAPIKey:         "test-key",
		ELKTimeoutSeconds: 2,
	}
	cleanup, err := Setup(cfg)
	if err != nil {
		t.Fatalf("Setup() with ELK error: %v", err)
	}
	defer cleanup()

	// 验证 ELK writer 初始化后日志写入不 panic
	log.Println("elk test entry")
}

// TestSetup_MultipleWriters 测试同时配置 stdout + 文件 + ELK 多个 writer 的组合场景
func TestSetup_MultipleWriters(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "multi.log")

	cfg := config.Config{
		LogToStdout:       true,
		LogFilePath:       logPath,
		ELKEnabled:        true,
		ELKEndpoint:       "http://127.0.0.1:9200",
		ELKIndex:          "test",
		ELKTimeoutSeconds: 1,
	}
	cleanup, err := Setup(cfg)
	if err != nil {
		t.Fatalf("Setup() error: %v", err)
	}

	log.Println("multi writer test")
	cleanup()

	// 验证文件 writer 正确接收到日志内容
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if !strings.Contains(string(content), "multi writer test") {
		t.Error("log file should contain the test entry")
	}
}

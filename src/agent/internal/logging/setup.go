package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"luoyi2026/agent/internal/config"
)

type elkWriter struct {
	endpoint string
	index    string
	apiKey   string
	client   *http.Client

	ch chan []byte
	wg sync.WaitGroup
}

type graylogWriter struct {
	transport string
	endpoint  string
	host      string
	level     int
	client    *http.Client

	conn   net.Conn
	connMu sync.Mutex

	ch chan []byte
	wg sync.WaitGroup
}

func Setup(cfg config.Config) (func(), error) {
	writers := make([]io.Writer, 0, 3)
	closers := make([]io.Closer, 0, 3)

	if cfg.LogToStdout {
		writers = append(writers, os.Stdout)
	}

	if cfg.LogFilePath != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.LogFilePath), 0o755); err != nil {
			return nil, err
		}
		file, err := os.OpenFile(cfg.LogFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, err
		}
		writers = append(writers, file)
		closers = append(closers, file)
	}

	if cfg.ELKEnabled && strings.TrimSpace(cfg.ELKEndpoint) != "" {
		writer := newELKWriter(cfg)
		writers = append(writers, writer)
		closers = append(closers, writer)
	}

	if cfg.GraylogEnabled && strings.TrimSpace(cfg.GraylogEndpoint) != "" {
		writer, err := newGraylogWriter(cfg)
		if err != nil {
			return nil, err
		}
		writers = append(writers, writer)
		closers = append(closers, writer)
	}

	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(io.MultiWriter(writers...))

	return func() {
		for _, closer := range closers {
			_ = closer.Close()
		}
	}, nil
}

func newELKWriter(cfg config.Config) *elkWriter {
	writer := &elkWriter{
		endpoint: strings.TrimSpace(cfg.ELKEndpoint),
		index:    cfg.ELKIndex,
		apiKey:   strings.TrimSpace(cfg.ELKAPIKey),
		client: &http.Client{
			Timeout: time.Duration(cfg.ELKTimeoutSeconds) * time.Second,
		},
		ch: make(chan []byte, 2048),
	}
	writer.wg.Add(1)
	go func() {
		defer writer.wg.Done()
		for payload := range writer.ch {
			writer.deliver(payload)
		}
	}()
	return writer
}

func (writer *elkWriter) Write(payload []byte) (int, error) {
	line := bytes.TrimSpace(payload)
	if len(line) == 0 {
		return len(payload), nil
	}
	entry := map[string]any{
		"@timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"service":    "luoyi-agent",
		"index":      writer.index,
		"message":    string(line),
	}
	body, err := json.Marshal(entry)
	if err != nil {
		return len(payload), nil
	}
	select {
	case writer.ch <- body:
	default:
	}
	return len(payload), nil
}

func (writer *elkWriter) Close() error {
	close(writer.ch)
	writer.wg.Wait()
	return nil
}

func (writer *elkWriter) deliver(payload []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), writer.client.Timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, writer.endpoint, bytes.NewReader(payload))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")
	if writer.apiKey != "" {
		request.Header.Set("Authorization", fmt.Sprintf("ApiKey %s", writer.apiKey))
	}
	response, err := writer.client.Do(request)
	if err != nil {
		return
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
}

func newGraylogWriter(cfg config.Config) (*graylogWriter, error) {
	writer := &graylogWriter{
		transport: strings.ToLower(strings.TrimSpace(cfg.GraylogTransport)),
		endpoint:  strings.TrimSpace(cfg.GraylogEndpoint),
		host:      strings.TrimSpace(cfg.GraylogHost),
		level:     cfg.GraylogLevel,
		client: &http.Client{
			Timeout: time.Duration(cfg.GraylogTimeoutSeconds) * time.Second,
		},
		ch: make(chan []byte, 2048),
	}
	if writer.host == "" {
		hostname, _ := os.Hostname()
		if strings.TrimSpace(hostname) == "" {
			writer.host = "luoyi-agent"
		} else {
			writer.host = hostname
		}
	}
	if writer.transport == "" {
		writer.transport = "udp"
	}

	if writer.transport != "http" {
		conn, err := net.DialTimeout(writer.transport, writer.endpoint, writer.client.Timeout)
		if err != nil {
			return nil, err
		}
		writer.conn = conn
	}

	writer.wg.Add(1)
	go func() {
		defer writer.wg.Done()
		for payload := range writer.ch {
			writer.deliver(payload)
		}
	}()
	return writer, nil
}

func (writer *graylogWriter) Write(payload []byte) (int, error) {
	line := strings.TrimSpace(string(payload))
	if line == "" {
		return len(payload), nil
	}

	gelf := map[string]any{
		"version":       "1.1",
		"host":          writer.host,
		"short_message": line,
		"timestamp":     float64(time.Now().UnixNano()) / float64(time.Second),
		"level":         writer.level,
		"_service":      "luoyi-agent",
	}
	body, err := json.Marshal(gelf)
	if err != nil {
		return len(payload), nil
	}

	select {
	case writer.ch <- body:
	default:
	}

	return len(payload), nil
}

func (writer *graylogWriter) Close() error {
	close(writer.ch)
	writer.wg.Wait()
	writer.connMu.Lock()
	defer writer.connMu.Unlock()
	if writer.conn != nil {
		_ = writer.conn.Close()
		writer.conn = nil
	}
	return nil
}

func (writer *graylogWriter) deliver(payload []byte) {
	if writer.transport == "http" {
		writer.deliverHTTP(payload)
		return
	}
	writer.deliverConn(payload)
}

func (writer *graylogWriter) deliverConn(payload []byte) {
	writer.connMu.Lock()
	defer writer.connMu.Unlock()

	if writer.conn == nil {
		conn, err := net.DialTimeout(writer.transport, writer.endpoint, writer.client.Timeout)
		if err != nil {
			return
		}
		writer.conn = conn
	}

	frame := payload
	if writer.transport == "tcp" {
		frame = append(payload, 0)
	}
	if _, err := writer.conn.Write(frame); err != nil {
		_ = writer.conn.Close()
		writer.conn = nil
	}
}

func (writer *graylogWriter) deliverHTTP(payload []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), writer.client.Timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, writer.endpoint, bytes.NewReader(payload))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := writer.client.Do(request)
	if err != nil {
		return
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
}

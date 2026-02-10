package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"luoyi2026/server/internal/config"
)

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

// Setup initialises log outputs based on config and returns a cleanup function.
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

func newGraylogWriter(cfg config.Config) (*graylogWriter, error) {
	w := &graylogWriter{
		transport: cfg.GraylogTransport,
		endpoint:  cfg.GraylogEndpoint,
		host:      cfg.GraylogHost,
		level:     cfg.GraylogLevel,
		client:    &http.Client{Timeout: time.Duration(cfg.GraylogTimeoutSeconds) * time.Second},
		ch:        make(chan []byte, 2048),
	}
	if w.host == "" {
		hostname, _ := os.Hostname()
		if strings.TrimSpace(hostname) == "" {
			w.host = "luoyi-server"
		} else {
			w.host = hostname
		}
	}
	if w.transport == "" {
		w.transport = "udp"
	}

	if w.transport != "http" {
		conn, err := net.DialTimeout(w.transport, w.endpoint, w.client.Timeout)
		if err != nil {
			return nil, err
		}
		w.conn = conn
	}

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		for payload := range w.ch {
			w.deliver(payload)
		}
	}()
	return w, nil
}

func (w *graylogWriter) Write(payload []byte) (int, error) {
	line := strings.TrimSpace(string(payload))
	if line == "" {
		return len(payload), nil
	}

	gelf := map[string]any{
		"version":       "1.1",
		"host":          w.host,
		"short_message": line,
		"timestamp":     float64(time.Now().UnixNano()) / float64(time.Second),
		"level":         w.level,
		"_service":      "luoyi-server",
	}
	body, err := json.Marshal(gelf)
	if err != nil {
		return len(payload), nil
	}

	select {
	case w.ch <- body:
	default:
	}
	return len(payload), nil
}

func (w *graylogWriter) Close() error {
	close(w.ch)
	w.wg.Wait()
	w.connMu.Lock()
	defer w.connMu.Unlock()
	if w.conn != nil {
		_ = w.conn.Close()
		w.conn = nil
	}
	return nil
}

func (w *graylogWriter) deliver(payload []byte) {
	if w.transport == "http" {
		w.deliverHTTP(payload)
		return
	}
	w.deliverConn(payload)
}

func (w *graylogWriter) deliverConn(payload []byte) {
	w.connMu.Lock()
	defer w.connMu.Unlock()

	if w.conn == nil {
		conn, err := net.DialTimeout(w.transport, w.endpoint, w.client.Timeout)
		if err != nil {
			return
		}
		w.conn = conn
	}

	frame := payload
	if w.transport == "tcp" {
		frame = append(payload, 0)
	}
	if _, err := w.conn.Write(frame); err != nil {
		_ = w.conn.Close()
		w.conn = nil
	}
}

func (w *graylogWriter) deliverHTTP(payload []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), w.client.Timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, w.endpoint, bytes.NewReader(payload))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := w.client.Do(request)
	if err != nil {
		return
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
}

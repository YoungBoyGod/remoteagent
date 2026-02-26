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

// routingWriter fans out each log line to all.log always, and to one of
// server.log / agent.log / error.log based on content keywords.
type routingWriter struct {
	all    io.Writer
	server io.Writer
	agent  io.Writer
	errW   io.Writer
}

func (r *routingWriter) Write(p []byte) (int, error) {
	_, _ = r.all.Write(p)
	line := string(p)
	lower := strings.ToLower(line)
	isLocalAccess := strings.Contains(line, "[L]")
	isRemoteAccess := strings.Contains(line, "[R]")
	hasAgentKeyword := strings.Contains(lower, "/api/v1/agent") ||
		strings.Contains(lower, "/agents/") ||
		strings.Contains(lower, "register") ||
		strings.Contains(lower, "heartbeat") ||
		strings.Contains(lower, "poll")

	switch {
	case strings.Contains(lower, "error") ||
		strings.Contains(lower, "failed") ||
		strings.Contains(lower, "panic"):
		_, _ = r.errW.Write(p)
	case isRemoteAccess:
		_, _ = r.agent.Write(p)
	case isLocalAccess:
		_, _ = r.server.Write(p)
	case hasAgentKeyword:
		_, _ = r.agent.Write(p)
	default:
		_, _ = r.server.Write(p)
	}
	return len(p), nil
}

func openLogFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

// Writer is the active log writer; set after Setup is called.
// Router can point gin.DefaultWriter here so GIN access logs go through the same routing.
var Writer io.Writer = os.Stdout

// Setup initialises log outputs based on config and returns a cleanup function.
func Setup(cfg config.Config) (func(), error) {
	writers := make([]io.Writer, 0, 3)
	closers := make([]io.Closer, 0, 3)

	if cfg.LogToStdout {
		writers = append(writers, os.Stdout)
	}

	if cfg.LogFilePath != "" {
		f, err := openLogFile(cfg.LogFilePath)
		if err != nil {
			return nil, err
		}
		writers = append(writers, f)
		closers = append(closers, f)
	}

	if cfg.LogDir != "" {
		if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
			return nil, err
		}
		open := func(name string) (*os.File, error) {
			return openLogFile(filepath.Join(cfg.LogDir, name))
		}
		all, err := open("all.log")
		if err != nil {
			return nil, err
		}
		srv, err := open("server.log")
		if err != nil {
			return nil, err
		}
		agt, err := open("agent.log")
		if err != nil {
			return nil, err
		}
		errF, err := open("error.log")
		if err != nil {
			return nil, err
		}
		closers = append(closers, all, srv, agt, errF)
		writers = append(writers, &routingWriter{all: all, server: srv, agent: agt, errW: errF})
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
	w := io.MultiWriter(writers...)
	log.SetOutput(w)
	Writer = w

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

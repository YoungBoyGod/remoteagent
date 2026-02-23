package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type counter struct {
	v atomic.Int64
}

func (c *counter) Add(value int64) {
	c.v.Add(value)
}

func (c *counter) Get() int64 {
	return c.v.Load()
}

type gauge struct {
	v atomic.Int64
}

func (g *gauge) Set(value int64) {
	g.v.Store(value)
}

func (g *gauge) Add(value int64) {
	g.v.Add(value)
}

func (g *gauge) Get() int64 {
	return g.v.Load()
}

type histogram struct {
	buckets []float64
	counts  []uint64
	sum     float64
	count   uint64
	mu      sync.Mutex
}

func newHistogram(buckets []float64) *histogram {
	sorted := append([]float64(nil), buckets...)
	sort.Float64s(sorted)
	return &histogram{
		buckets: sorted,
		counts:  make([]uint64, len(sorted)),
	}
}

func (h *histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.count++
	h.sum += value
	for i, bucket := range h.buckets {
		if value <= bucket {
			h.counts[i]++
		}
	}
}

func (h *histogram) Snapshot() (buckets []float64, counts []uint64, sum float64, count uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	buckets = append([]float64(nil), h.buckets...)
	counts = append([]uint64(nil), h.counts...)
	sum = h.sum
	count = h.count
	return buckets, counts, sum, count
}

type Metrics struct {
	registerTotal      counter
	registerFailures   counter
	heartbeatTotal     counter
	heartbeatFailures  counter
	pollTotal          counter
	pollFailures       counter
	tasksStartedTotal  counter
	tasksFinishedTotal counter
	tasksFailedTotal   counter
	tasksCanceledTotal counter
	pendingQueueSize   gauge
	runningTasks       gauge
	httpLatencyMs      *histogram

	startTime time.Time
}

func NewMetrics() *Metrics {
	return &Metrics{
		httpLatencyMs: newHistogram([]float64{10, 25, 50, 100, 200, 500, 1000, 2000, 5000}),
		startTime:     time.Now(),
	}
}

func (m *Metrics) IncRegister(ok bool) {
	m.registerTotal.Add(1)
	if !ok {
		m.registerFailures.Add(1)
	}
}

func (m *Metrics) IncHeartbeat(ok bool) {
	m.heartbeatTotal.Add(1)
	if !ok {
		m.heartbeatFailures.Add(1)
	}
}

func (m *Metrics) IncPoll(ok bool) {
	m.pollTotal.Add(1)
	if !ok {
		m.pollFailures.Add(1)
	}
}

func (m *Metrics) IncTaskStarted() {
	m.tasksStartedTotal.Add(1)
	m.runningTasks.Add(1)
}

func (m *Metrics) IncTaskFinished(status string) {
	m.tasksFinishedTotal.Add(1)
	m.runningTasks.Add(-1)
	if m.runningTasks.Get() < 0 {
		m.runningTasks.Set(0)
	}
	switch status {
	case "failed":
		m.tasksFailedTotal.Add(1)
	case "canceled":
		m.tasksCanceledTotal.Add(1)
	}
}

func (m *Metrics) SetPendingQueueSize(size int) {
	if size < 0 {
		size = 0
	}
	m.pendingQueueSize.Set(int64(size))
}

func (m *Metrics) ObserveHTTP(duration time.Duration) {
	m.httpLatencyMs.Observe(float64(duration.Milliseconds()))
}

func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = writer.Write([]byte(m.RenderPrometheus()))
	})
}

func (m *Metrics) RenderPrometheus() string {
	var builder strings.Builder
	appendCounter(&builder, "agent_register_total", "Total register attempts", m.registerTotal.Get())
	appendCounter(&builder, "agent_register_failures_total", "Total register failures", m.registerFailures.Get())
	appendCounter(&builder, "agent_heartbeat_total", "Total heartbeat attempts", m.heartbeatTotal.Get())
	appendCounter(&builder, "agent_heartbeat_failures_total", "Total heartbeat failures", m.heartbeatFailures.Get())
	appendCounter(&builder, "agent_poll_total", "Total poll attempts", m.pollTotal.Get())
	appendCounter(&builder, "agent_poll_failures_total", "Total poll failures", m.pollFailures.Get())
	appendCounter(&builder, "agent_tasks_started_total", "Total tasks started", m.tasksStartedTotal.Get())
	appendCounter(&builder, "agent_tasks_finished_total", "Total tasks finished", m.tasksFinishedTotal.Get())
	appendCounter(&builder, "agent_tasks_failed_total", "Total tasks failed", m.tasksFailedTotal.Get())
	appendCounter(&builder, "agent_tasks_canceled_total", "Total tasks canceled", m.tasksCanceledTotal.Get())
	appendGauge(&builder, "agent_pending_queue_size", "Pending report queue size", m.pendingQueueSize.Get())
	appendGauge(&builder, "agent_running_tasks", "Current running tasks", m.runningTasks.Get())
	appendGauge(&builder, "agent_uptime_seconds", "Agent process uptime", int64(time.Since(m.startTime).Seconds()))
	appendHistogram(&builder, "agent_http_request_duration_ms", "HTTP request latency in milliseconds", m.httpLatencyMs)
	return builder.String()
}

func appendCounter(builder *strings.Builder, name string, help string, value int64) {
	builder.WriteString("# HELP ")
	builder.WriteString(name)
	builder.WriteString(" ")
	builder.WriteString(help)
	builder.WriteString("\n# TYPE ")
	builder.WriteString(name)
	builder.WriteString(" counter\n")
	builder.WriteString(fmt.Sprintf("%s %d\n", name, value))
}

func appendGauge(builder *strings.Builder, name string, help string, value int64) {
	builder.WriteString("# HELP ")
	builder.WriteString(name)
	builder.WriteString(" ")
	builder.WriteString(help)
	builder.WriteString("\n# TYPE ")
	builder.WriteString(name)
	builder.WriteString(" gauge\n")
	builder.WriteString(fmt.Sprintf("%s %d\n", name, value))
}

func appendHistogram(builder *strings.Builder, name string, help string, histogram *histogram) {
	builder.WriteString("# HELP ")
	builder.WriteString(name)
	builder.WriteString(" ")
	builder.WriteString(help)
	builder.WriteString("\n# TYPE ")
	builder.WriteString(name)
	builder.WriteString(" histogram\n")

	buckets, counts, sum, count := histogram.Snapshot()
	for i := range buckets {
		builder.WriteString(fmt.Sprintf("%s_bucket{le=\"%g\"} %d\n", name, buckets[i], counts[i]))
	}
	builder.WriteString(fmt.Sprintf("%s_bucket{le=\"+Inf\"} %d\n", name, count))
	builder.WriteString(fmt.Sprintf("%s_sum %f\n", name, sum))
	builder.WriteString(fmt.Sprintf("%s_count %d\n", name, count))
}

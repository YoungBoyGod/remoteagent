package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

func collectDeviceInfo() deviceInfo {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown"
	}
	return deviceInfo{
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		IP:       detectLocalIP(),
	}
}

func detectLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	udpAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return "127.0.0.1"
	}
	if udpAddr.IP == nil {
		return "127.0.0.1"
	}
	return udpAddr.IP.String()
}

func collectMetrics() metricsInfo {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	memMB := float64(memory.Alloc) / 1024.0 / 1024.0
	return metricsInfo{
		CPUPercent:  0,
		MemPercent:  memMB,
		DiskPercent: 0,
	}
}

func writeFileAtomic(path string, content []byte, mode os.FileMode) error {
	tempPath := fmt.Sprintf("%s.tmp.%d", path, time.Now().UnixNano())
	if err := os.WriteFile(tempPath, content, mode); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func newUUIDLike() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw)
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32])
}

func newEventID() string {
	return "evt-" + randomHex(8)
}

func randomHex(length int) string {
	if length <= 0 {
		length = 8
	}
	raw := make([]byte, length)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}

func nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > 60*time.Second {
		return 60 * time.Second
	}
	if next < time.Second {
		return time.Second
	}
	return next
}

func backoffWithJitter(delay time.Duration) time.Duration {
	if delay <= 0 {
		delay = time.Second
	}
	jitterRaw := make([]byte, 1)
	_, _ = rand.Read(jitterRaw)
	jitterMs := int(jitterRaw[0]) % 500
	return delay + time.Duration(jitterMs)*time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func max(current int, fallback int) int {
	if current <= 0 {
		return fallback
	}
	return current
}

func readStringMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

// scrapeNodeExporter 从本地 node_exporter 采集 Prometheus 格式指标
func scrapeNodeExporter() string {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9100/metrics")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return ""
	}
	// 过滤掉注释行和空行，只保留指标数据行，减小传输体积
	var sb strings.Builder
	for _, line := range strings.Split(string(body), "\n") {
		if line == "" || line[0] == '#' {
			continue
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	return sb.String()
}

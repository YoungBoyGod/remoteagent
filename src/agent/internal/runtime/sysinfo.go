package runtime

import (
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"strings"
	"syscall"
)

func logSystemInfo() {
	hostname, _ := os.Hostname()
	log.Printf("── system info ──")
	log.Printf("  hostname:  %s", hostname)
	log.Printf("  os/arch:   %s/%s", runtime.GOOS, runtime.GOARCH)
	log.Printf("  cpu:       %d cores", runtime.NumCPU())
	log.Printf("  memory:    %s", formatMemory())
	log.Printf("  disk:      %s", formatDisk("/"))
	log.Printf("  ip:        %s", getLocalIPs())
	log.Printf("  external:  %s", detectExternalIP())
}

func formatMemory() string {
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err != nil {
		return "unknown"
	}
	total := si.Totalram * uint64(si.Unit)
	free := (si.Freeram + si.Bufferram) * uint64(si.Unit)
	return fmt.Sprintf("total %s, available %s", humanBytes(total), humanBytes(free))
}

func formatDisk(path string) string {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return "unknown"
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	return fmt.Sprintf("total %s, free %s", humanBytes(total), humanBytes(free))
}

func getLocalIPs() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown"
	}
	var ips []string
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			ips = append(ips, ipNet.IP.String())
		}
	}
	if len(ips) == 0 {
		return "none"
	}
	return strings.Join(ips, ", ")
}

func humanBytes(b uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(MB))
	default:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(KB))
	}
}

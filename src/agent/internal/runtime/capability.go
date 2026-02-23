package runtime

import (
	"os/exec"
	"runtime"
	"strings"
	"syscall"
)

// agentCapabilityInfo Agent 结构化能力信息
type agentCapabilityInfo struct {
	CPUCores        int      `json:"cpu_cores"`
	MemoryBytes     uint64   `json:"memory_bytes"`
	DiskBytes       uint64   `json:"disk_bytes"`
	GPUList         []string `json:"gpu_list"`
	DockerAvailable bool     `json:"docker_available"`
	CUDAVersion     string   `json:"cuda_version"`
}

// collectCapability 采集 Agent 结构化能力信息
func collectCapability() agentCapabilityInfo {
	info := agentCapabilityInfo{
		CPUCores: runtime.NumCPU(),
		GPUList:  make([]string, 0),
	}

	// 内存：通过 syscall.Sysinfo 获取
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err == nil {
		info.MemoryBytes = si.Totalram * uint64(si.Unit)
	}

	// 磁盘：通过 syscall.Statfs 获取根分区
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		info.DiskBytes = stat.Blocks * uint64(stat.Bsize)
	}

	// GPU：尝试执行 nvidia-smi
	if out, err := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader").Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			name := strings.TrimSpace(line)
			if name != "" {
				info.GPUList = append(info.GPUList, name)
			}
		}
	}

	// Docker：尝试执行 docker version
	if out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output(); err == nil {
		version := strings.TrimSpace(string(out))
		if version != "" {
			info.DockerAvailable = true
		}
	}

	// CUDA：尝试执行 nvcc --version
	if out, err := exec.Command("nvcc", "--version").Output(); err == nil {
		info.CUDAVersion = parseCUDAVersion(string(out))
	}

	return info
}

// parseCUDAVersion 从 nvcc --version 输出中解析 CUDA 版本号
func parseCUDAVersion(output string) string {
	// nvcc 输出格式示例：
	// Cuda compilation tools, release 12.2, V12.2.140
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "release") {
			parts := strings.Split(line, "release ")
			if len(parts) >= 2 {
				version := strings.Split(parts[1], ",")[0]
				return strings.TrimSpace(version)
			}
		}
	}
	return ""
}

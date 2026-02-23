package runtime

import "sync"

// concurrencyController 管理 shared/exclusive 并发控制
type concurrencyController struct {
	mu               sync.Mutex
	maxConcurrent    int
	runningShared    int
	runningExclusive bool
	draining         bool // 等待 shared 完成以执行 exclusive
}

// agentCapacity 当前容量信息（用于上报）
type agentCapacity struct {
	MaxConcurrent    int  `json:"max_concurrent"`
	RunningShared    int  `json:"running_shared"`
	RunningExclusive bool `json:"running_exclusive"`
}

func newConcurrencyController(maxConcurrent int) *concurrencyController {
	if maxConcurrent <= 0 {
		maxConcurrent = 4
	}
	return &concurrencyController{maxConcurrent: maxConcurrent}
}

// canAccept 判断是否可以接受新任务
func (cc *concurrencyController) canAccept(execMode string) bool {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	return cc.canAcceptLocked(execMode)
}

func (cc *concurrencyController) canAcceptLocked(execMode string) bool {
	if execMode == "exclusive" {
		// exclusive 任务：没有 shared 在跑，也没有 exclusive 在跑
		return cc.runningShared == 0 && !cc.runningExclusive
	}
	// shared 任务：没有 exclusive 在跑，shared 数量未满，且不在 draining 状态
	return !cc.runningExclusive && cc.runningShared < cc.maxConcurrent && !cc.draining
}

// acquire 占用一个槽位，成功返回 true
func (cc *concurrencyController) acquire(execMode string) bool {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if !cc.canAcceptLocked(execMode) {
		return false
	}
	if execMode == "exclusive" {
		cc.runningExclusive = true
	} else {
		cc.runningShared++
	}
	return true
}

// release 释放一个槽位
func (cc *concurrencyController) release(execMode string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if execMode == "exclusive" {
		cc.runningExclusive = false
	} else {
		cc.runningShared--
		if cc.runningShared < 0 {
			cc.runningShared = 0
		}
	}
	// shared 全部完成后清除 draining 标记
	if cc.draining && cc.runningShared == 0 {
		cc.draining = false
	}
}

// setDraining 设置 draining 状态，阻止新 shared 任务进入
func (cc *concurrencyController) setDraining() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.draining = true
}

// capacity 返回当前容量信息
func (cc *concurrencyController) capacity() agentCapacity {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	return agentCapacity{
		MaxConcurrent:    cc.maxConcurrent,
		RunningShared:    cc.runningShared,
		RunningExclusive: cc.runningExclusive,
	}
}

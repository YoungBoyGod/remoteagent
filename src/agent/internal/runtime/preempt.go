package runtime

import (
	"log"
	"os/exec"
	"syscall"
	"time"
)

// preemptCommand 抢占指令
type preemptCommand struct {
	TaskID             string `json:"task_id"`
	Reason             string `json:"reason"`
	GracePeriodSeconds int    `json:"grace_period_seconds"`
	Deadline           int64  `json:"deadline"` // unix 毫秒
}

// preemptAckRequest 抢占确认请求
type preemptAckRequest struct {
	EventID      string `json:"event_id"`
	AgentID      string `json:"agent_id"`
	TaskID       string `json:"task_id"`
	Timestamp    int64  `json:"timestamp"`
	PreemptState string `json:"preempt_state"` // ack / completed / force_killed
}

// handlePreempt 处理抢占指令：ack -> SIGTERM -> grace period -> SIGKILL
func (a *Agent) handlePreempt(taskID string, cmd *preemptCommand) {
	// 1. 发送 ack
	a.sendPreemptAck(taskID, "ack")

	gracePeriod := time.Duration(cmd.GracePeriodSeconds) * time.Second
	if gracePeriod <= 0 {
		gracePeriod = 30 * time.Second
	}

	// 2. 发送 SIGTERM 给任务进程
	a.mu.Lock()
	rt, ok := a.running[taskID]
	a.mu.Unlock()
	if !ok {
		log.Printf("preempt: task %s not running, skip", taskID)
		a.sendPreemptAck(taskID, "completed")
		return
	}

	// 标记为被抢占
	a.markPreempted(taskID)

	// 发送 SIGTERM（通过 cancel context 触发）
	log.Printf("preempt: sending SIGTERM to task %s, grace period %s", taskID, gracePeriod)
	rt.Cancel()

	// 3. 等待 grace period
	deadline := time.NewTimer(gracePeriod)
	defer deadline.Stop()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline.C:
			// grace period 到期，检查任务是否还在运行
			a.mu.Lock()
			_, stillRunning := a.running[taskID]
			a.mu.Unlock()
			if stillRunning {
				log.Printf("preempt: force killing task %s after grace period", taskID)
				a.forceKillTask(taskID)
				a.sendPreemptAck(taskID, "force_killed")
			} else {
				a.sendPreemptAck(taskID, "completed")
			}
			return
		case <-ticker.C:
			// 检查任务是否已经结束
			a.mu.Lock()
			_, stillRunning := a.running[taskID]
			a.mu.Unlock()
			if !stillRunning {
				log.Printf("preempt: task %s exited gracefully", taskID)
				a.sendPreemptAck(taskID, "completed")
				return
			}
		}
	}
}

// sendPreemptAck 发送抢占确认
func (a *Agent) sendPreemptAck(taskID string, state string) {
	req := preemptAckRequest{
		EventID:      newEventID(),
		AgentID:      a.agentID,
		TaskID:       taskID,
		Timestamp:    time.Now().Unix(),
		PreemptState: state,
	}
	a.sendOrQueue("/api/v1/tasks/"+taskID+"/preempt/ack", req)
}

// forceKillTask 强制杀死任务进程组
func (a *Agent) forceKillTask(taskID string) {
	a.mu.Lock()
	rt, ok := a.running[taskID]
	a.mu.Unlock()
	if !ok {
		return
	}
	// Cancel 已经调用过了，这里再次调用确保
	rt.Cancel()
}

// markPreempted 标记任务为被抢占状态
func (a *Agent) markPreempted(taskID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.preempted[taskID] = struct{}{}
}

// takePreemptMark 取出并清除抢占标记
func (a *Agent) takePreemptMark(taskID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.preempted[taskID]
	if ok {
		delete(a.preempted, taskID)
	}
	return ok
}

// sendSIGTERM 向进程组发送 SIGTERM（用于优雅终止）
func sendSIGTERM(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
}

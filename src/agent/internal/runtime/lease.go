package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// 租约续期间隔：租约剩余时间的 1/3，最小 5 秒，最大 30 秒
const (
	minLeaseRenewInterval = 5 * time.Second
	maxLeaseRenewInterval = 30 * time.Second
)

// taskHeartbeatRequest 任务续租请求
type taskHeartbeatRequest struct {
	AgentID string `json:"agent_id"`
}

// taskHeartbeatResponse 任务续租响应
type taskHeartbeatResponse struct {
	LeasedUntil    int64           `json:"leased_until"`
	PreemptCommand *preemptCommand `json:"preempt_command,omitempty"`
}

// leaseRenewalLoop 在任务执行期间周期性续租
func (a *Agent) leaseRenewalLoop(ctx context.Context, taskID string, leasedUntilMs int64) {
	for {
		interval := calcRenewInterval(leasedUntilMs)
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}

		newLease, preempt, err := a.renewTaskLease(ctx, taskID)
		if err != nil {
			log.Printf("lease renewal failed for task %s: %v", taskID, err)
			// 续租失败不立即放弃，下次重试
			continue
		}
		leasedUntilMs = newLease

		// 更新本地队列中的租约时间
		if a.db != nil {
			dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, _ = a.db.ExecContext(dbCtx, `
				UPDATE local_tasks SET leased_until_ms = ? WHERE task_id = ?
			`, leasedUntilMs, taskID)
			cancel()
		}

		// 如果收到抢占指令，处理抢占
		if preempt != nil {
			log.Printf("received preempt command for task %s: reason=%s grace=%ds",
				taskID, preempt.Reason, preempt.GracePeriodSeconds)
			go a.handlePreempt(taskID, preempt)
			return
		}
	}
}

// renewTaskLease 调用 Server 续租 API
func (a *Agent) renewTaskLease(ctx context.Context, taskID string) (int64, *preemptCommand, error) {
	req := taskHeartbeatRequest{AgentID: a.agentID}
	envelope, err := a.postAuthJSON(ctx, "/api/v1/tasks/"+taskID+"/heartbeat", req)
	if err != nil {
		return 0, nil, err
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return 0, nil, fmt.Errorf("empty heartbeat response for task %s", taskID)
	}

	var resp taskHeartbeatResponse
	if err := json.Unmarshal(envelope.Data, &resp); err != nil {
		return 0, nil, fmt.Errorf("decode heartbeat response: %w", err)
	}
	return resp.LeasedUntil, resp.PreemptCommand, nil
}

// calcRenewInterval 计算续租间隔：租约剩余时间的 1/3
func calcRenewInterval(leasedUntilMs int64) time.Duration {
	remaining := time.Until(time.UnixMilli(leasedUntilMs))
	interval := remaining / 3
	if interval < minLeaseRenewInterval {
		interval = minLeaseRenewInterval
	}
	if interval > maxLeaseRenewInterval {
		interval = maxLeaseRenewInterval
	}
	return interval
}

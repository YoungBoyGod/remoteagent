package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"
)

func (a *Agent) heartbeatLoop(ctx context.Context) {
	backoff := time.Second
	for {
		err := a.sendHeartbeat(ctx)
		if err == nil {
			backoff = time.Second
			if !sleepContext(ctx, a.heartbeatInterval) {
				return
			}
			continue
		}
		if errors.Is(err, errUnauthorized) {
			a.triggerReauth()
			return
		}
		log.Printf("heartbeat failed: %v", err)
		if !sleepContext(ctx, backoffWithJitter(backoff)) {
			return
		}
		backoff = nextBackoff(backoff)
	}
}

func (a *Agent) pollLoop(ctx context.Context) {
	backoff := time.Second
	for {
		message, err := a.pollOnce(ctx)
		if err == nil {
			backoff = time.Second
			if message == nil {
				continue
			}
			a.handlePollMessage(message)
			continue
		}
		if errors.Is(err, errUnauthorized) {
			a.triggerReauth()
			return
		}
		if errors.Is(err, context.Canceled) {
			return
		}
		log.Printf("poll failed: %v", err)
		if !sleepContext(ctx, backoffWithJitter(backoff)) {
			return
		}
		backoff = nextBackoff(backoff)
	}
}

func (a *Agent) handlePollMessage(message *pollMessage) {
	switch message.Type {
	case "task":
		var payload taskPayload
		if err := json.Unmarshal(message.Data, &payload); err != nil {
			log.Printf("invalid task payload: %v", err)
			return
		}
		if payload.TaskID == "" || payload.TaskType != "command" || payload.Payload.Command == "" {
			log.Printf("ignore invalid task message")
			return
		}
		go a.runTask(payload)
	case "control":
		var payload controlPayload
		if err := json.Unmarshal(message.Data, &payload); err != nil {
			log.Printf("invalid control payload: %v", err)
			return
		}
		a.handleControl(payload)
	default:
		log.Printf("ignore unknown message type: %s", message.Type)
	}
}

func (a *Agent) handleControl(payload controlPayload) {
	switch payload.Action {
	case "refresh_token":
		log.Printf("control received: refresh_token")
		a.triggerReauth()
	case "shutdown":
		log.Printf("control received: shutdown")
		a.requestShutdown("control shutdown")
	case "reload_config":
		log.Printf("control received: reload_config")
		a.ReloadConfig()
	case "cancel_task", "cancel":
		taskID := strings.TrimSpace(readStringMap(payload.Payload, "task_id"))
		if taskID == "" {
			log.Printf("control cancel ignored: missing task_id")
			return
		}
		if a.cancelTaskFromControl(taskID) {
			log.Printf("control cancel accepted: %s", taskID)
			return
		}
		log.Printf("control cancel ignored: task not running %s", taskID)
	default:
		log.Printf("control ignored: %s", payload.Action)
	}
}

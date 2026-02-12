package runtime

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (a *Agent) initialize() error {
	_ = a.setState(StateInit)
	logSystemInfo()
	log.Printf("  device:    %s", a.cfg.DeviceCode)
	log.Printf("  server:    %s", a.cfg.ServerAddr)
	log.Printf("  data_dir:  %s", a.cfg.DataDir)
	// 确保数据目录和 SQLite 父目录存在
	if err := os.MkdirAll(a.cfg.DataDir, 0o755); err != nil {
		return err
	}
	if a.cfg.SQLitePath != "none" {
		if dir := filepath.Dir(a.cfg.SQLitePath); dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
		}
		db, err := openSQLite(a.cfg.SQLitePath)
		if err != nil {
			log.Printf("[WARN] sqlite open failed, falling back to JSON storage: %v", err)
		} else {
			a.db = db
		}
	}
	if err := a.initLocalQueue(); err != nil {
		return err
	}
	if err := a.migrateJSONStoreIfNeeded(); err != nil {
		return err
	}
	agentID, err := a.loadOrCreateAgentID()
	if err != nil {
		return err
	}
	a.agentID = agentID
	if err := a.loadTasks(); err != nil {
		return err
	}
	if err := a.loadPending(); err != nil {
		return err
	}
	if a.obs != nil {
		a.obs.SetPendingQueueSize(len(a.pending))
	}
	a.recoverRunningTasks()
	log.Printf("agent initialized: agent_id=%s device_code=%s server=%s", a.agentID, a.cfg.DeviceCode, a.cfg.ServerAddr)
	return nil
}

func (a *Agent) recoverRunningTasks() {
	now := time.Now().Unix()
	for _, task := range a.tasks {
		if task.Status != "running" {
			continue
		}
		task.Status = "failed"
		task.FinishedAt = now
		task.ExitCode = -1
		task.LastError = "agent restarted while task running"
		task.UpdatedAt = now

		statusReq := taskStatusRequest{
			EventID:   newEventID(),
			AgentID:   a.agentID,
			TaskID:    task.TaskID,
			Status:    "failed",
			Timestamp: now,
			Attempt:   max(task.Attempt, 1),
		}
		a.enqueuePending("/api/v1/agent/task/status", statusReq)

		reportReq := taskReportRequest{
			EventID:    newEventID(),
			AgentID:    a.agentID,
			TaskID:     task.TaskID,
			Status:     "failed",
			StartedAt:  task.StartedAt,
			FinishedAt: now,
			Result: reportResult{
				ExitCode: -1,
				Stdout:   "",
				Stderr:   "agent restarted while task running",
			},
		}
		a.enqueuePending("/api/v1/agent/task/report", reportReq)
	}
	a.mu.Lock()
	_ = a.persistTasksLocked()
	a.mu.Unlock()
}

func (a *Agent) loadOrCreateAgentID() (string, error) {
	agentID, err := a.loadAgentIDFromDB()
	if err != nil {
		return "", err
	}
	if agentID != "" {
		return agentID, nil
	}

	content, err := os.ReadFile(a.paths.agentIDPath)
	if err == nil {
		id := strings.TrimSpace(string(content))
		if id != "" {
			if saveErr := a.saveAgentIDToDB(id); saveErr != nil {
				return "", saveErr
			}
			return id, nil
		}
	}
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	id := newUUIDLike()
	if err := a.saveAgentIDToDB(id); err != nil {
		return "", err
	}
	if err := writeFileAtomic(a.paths.agentIDPath, []byte(id+"\n"), 0o644); err != nil {
		return "", err
	}
	return id, nil
}

func (a *Agent) loadTasks() error {
	if a.db != nil {
		return a.loadTasksFromDB()
	}
	return a.loadTasksFromJSON()
}

func (a *Agent) loadTasksFromJSON() error {
	data, err := os.ReadFile(a.paths.tasksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var file taskStoreFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}
	for _, task := range file.Tasks {
		if task == nil || task.TaskID == "" {
			continue
		}
		a.tasks[task.TaskID] = task
	}
	return nil
}

func (a *Agent) loadPending() error {
	if a.db != nil {
		return a.loadPendingFromDB()
	}
	return a.loadPendingFromJSON()
}

func (a *Agent) loadPendingFromJSON() error {
	data, err := os.ReadFile(a.paths.pendingPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var pending []queuedRequest
	if err := json.Unmarshal(data, &pending); err != nil {
		return err
	}
	a.pending = pending
	return nil
}

func (a *Agent) persistTasksLocked() error {
	if a.db != nil {
		if err := a.persistTasksToDBLocked(); err != nil {
			return err
		}
	}

	if a.paths.tasksPath == "" {
		return nil
	}

	items := make([]*taskRecord, 0, len(a.tasks))
	for _, record := range a.tasks {
		cloned := *record
		items = append(items, &cloned)
	}
	sort.Slice(items, func(left int, right int) bool {
		return items[left].TaskID < items[right].TaskID
	})
	payload, err := json.MarshalIndent(taskStoreFile{Tasks: items}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(a.paths.tasksPath, payload, 0o644)
}

func (a *Agent) persistPendingLocked() error {
	if a.db != nil {
		if err := a.persistPendingToDBLocked(); err != nil {
			return err
		}
	}

	if a.paths.pendingPath == "" {
		return nil
	}

	payload, err := json.MarshalIndent(a.pending, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(a.paths.pendingPath, payload, 0o644)
}

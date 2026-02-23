package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrTaskNotFound      = errors.New("task not found")
	ErrTaskStateConflict = errors.New("task state conflict")
	ErrTaskAgentMismatch = errors.New("task agent mismatch")
)

func RequestTaskPreempt(db *sql.DB, taskID string, gracePeriodSeconds int, reason string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if gracePeriodSeconds <= 0 {
		gracePeriodSeconds = 30
	}

	var deadline time.Time
	err := db.QueryRowContext(
		ctx,
		`update tasks
		 set status='canceling',
		     preempt_state='requested',
		     preempt_requested_at=now(),
		     preempt_deadline=now() + ($2 * interval '1 second'),
		     preempt_reason=$3,
		     updated_at=now()
		 where task_id=$1
		   and status='running'
		   and preemptible=true
		 returning preempt_deadline`,
		taskID,
		gracePeriodSeconds,
		reason,
	).Scan(&deadline)
	if err == nil {
		return deadline.Unix(), nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	// 二次判断：不存在 / 幂等成功 / 状态冲突
	var status string
	var preemptible bool
	var preemptState string
	err = db.QueryRowContext(
		ctx,
		`select status, preemptible, preempt_state
		 from tasks
		 where task_id=$1`,
		taskID,
	).Scan(&status, &preemptible, &preemptState)
	if err == sql.ErrNoRows {
		return 0, ErrTaskNotFound
	}
	if err != nil {
		return 0, err
	}

	if status == "canceling" && (preemptState == "requested" || preemptState == "acknowledged") {
		var existingDeadline time.Time
		err = db.QueryRowContext(ctx,
			`select coalesce(preempt_deadline, now())
			 from tasks
			 where task_id=$1`,
			taskID,
		).Scan(&existingDeadline)
		if err != nil {
			return 0, err
		}
		return existingDeadline.Unix(), nil
	}

	_ = preemptible
	return 0, ErrTaskStateConflict
}

func AckTaskPreempt(db *sql.DB, taskID, agentID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.ExecContext(
		ctx,
		`update tasks
		 set preempt_state='acknowledged',
		     updated_at=now()
		 where task_id=$1
		   and agent_id=$2
		   and status='canceling'
		   and preempt_state in ('requested','acknowledged')`,
		taskID,
		agentID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows > 0 {
		return nil
	}

	// 二次判断：不存在 / agent 不匹配 / 状态冲突
	var dbAgentID, status, preemptState string
	err = db.QueryRowContext(
		ctx,
		`select agent_id, status, preempt_state
		 from tasks
		 where task_id=$1`,
		taskID,
	).Scan(&dbAgentID, &status, &preemptState)
	if err == sql.ErrNoRows {
		return ErrTaskNotFound
	}
	if err != nil {
		return err
	}

	if dbAgentID != agentID {
		return ErrTaskAgentMismatch
	}
	if status == "canceling" && preemptState == "acknowledged" {
		return nil
	}
	return ErrTaskStateConflict
}


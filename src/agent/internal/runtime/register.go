package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func (a *Agent) registerUntilSuccess(ctx context.Context) error {
	_ = a.setState(StateRegistering)
	backoff := time.Second
	for {
		err := a.registerOnce(ctx)
		if a.obs != nil {
			a.obs.IncRegister(err == nil)
		}
		if err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) {
			return err
		}
		log.Printf("register failed: %v", err)
		if !sleepContext(ctx, backoffWithJitter(backoff)) {
			return context.Canceled
		}
		backoff = nextBackoff(backoff)
	}
}

func (a *Agent) registerOnce(ctx context.Context) error {
	request := registerRequest{
		AgentID:      a.agentID,
		DeviceCode:   a.cfg.DeviceCode,
		AgentVersion: a.cfg.AgentVersion,
		TenantID:     a.cfg.TenantID,
		Device:       collectDeviceInfo(),
		Labels: map[string]string{
			"runtime": "go",
		},
		Capabilities: []string{"command_exec"},
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	requestURL := strings.TrimRight(a.cfg.ServerAddr, "/") + "/api/v1/agent/register"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Register-Token", a.cfg.RegisterToken)

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("register unauthorized")
	}
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("register status %d: %s", resp.StatusCode, string(body))
	}

	var envelope apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	var data registerData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return err
	}
	if data.Token == "" {
		return fmt.Errorf("empty token in register response")
	}

	a.mu.Lock()
	a.token = data.Token
	if data.AgentID != "" && data.AgentID != a.agentID {
		a.mu.Unlock()
		return fmt.Errorf("register rejected: server agent_id mismatch (local=%s server=%s)", a.agentID, data.AgentID)
	}
	if data.HeartbeatInterval > 0 {
		a.heartbeatInterval = time.Duration(data.HeartbeatInterval) * time.Second
	} else {
		a.heartbeatInterval = defaultHeartbeatInterval
	}
	if data.PollTimeout > 0 {
		a.pollTimeout = time.Duration(data.PollTimeout) * time.Second
	}
	a.mu.Unlock()

	log.Printf(
		"registered success: agent_id=%s heartbeat=%ds poll_timeout=%ds",
		a.agentID,
		int(a.heartbeatInterval/time.Second),
		int(a.pollTimeout/time.Second),
	)
	return nil
}

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

func (a *Agent) sendHeartbeat(ctx context.Context) error {
	runningTasks := a.runningTaskIDs()
	req := heartbeatRequest{
		AgentID:      a.agentID,
		Timestamp:    time.Now().Unix(),
		Metrics:      collectMetrics(),
		RunningTasks: runningTasks,
	}
	_, err := a.postAuthJSON(ctx, "/api/v1/agent/heartbeat", req)
	return err
}

func (a *Agent) pollOnce(ctx context.Context) (*pollMessage, error) {
	token := a.getToken()
	if token == "" {
		return nil, errUnauthorized
	}
	requestURL := strings.TrimRight(a.cfg.ServerAddr, "/") + "/api/v1/agent/poll?agent_id=" + a.agentID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errUnauthorized
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return nil, fmt.Errorf("poll status %d", resp.StatusCode)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("poll bad status %d: %s", resp.StatusCode, string(body))
	}

	var envelope apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil, nil
	}
	var message pollMessage
	if err := json.Unmarshal(envelope.Data, &message); err != nil {
		return nil, err
	}
	return &message, nil
}

func (a *Agent) postAuthJSON(ctx context.Context, path string, payload any) (apiEnvelope, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return apiEnvelope{}, err
	}
	return a.postAuthRaw(ctx, path, body)
}

func (a *Agent) postAuthRaw(ctx context.Context, path string, body []byte) (apiEnvelope, error) {
	token := a.getToken()
	if token == "" {
		return apiEnvelope{}, errUnauthorized
	}
	requestURL := strings.TrimRight(a.cfg.ServerAddr, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return apiEnvelope{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return apiEnvelope{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return apiEnvelope{}, errUnauthorized
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return apiEnvelope{}, httpStatusError{StatusCode: resp.StatusCode, Body: string(payload)}
	}
	if resp.StatusCode >= http.StatusBadRequest {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return apiEnvelope{}, httpStatusError{StatusCode: resp.StatusCode, Body: string(payload)}
	}
	var envelope apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return apiEnvelope{}, err
	}
	return envelope, nil
}

func (a *Agent) sendOrQueue(path string, payload any) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := a.postAuthJSONWithRetry(ctx, path, payload, 3)
	if err == nil {
		return
	}
	if errors.Is(err, errUnauthorized) {
		a.triggerReauth()
	}
	a.enqueuePending(path, payload)
}

func (a *Agent) enqueuePending(path string, payload any) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		log.Printf("pending marshal failed: %v", err)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pending = append(a.pending, queuedRequest{
		Path:    path,
		Body:    encoded,
		AddedAt: time.Now().Unix(),
	})
	if len(a.pending) > 1000 {
		a.pending = a.pending[len(a.pending)-1000:]
	}
	if err := a.persistPendingLocked(); err != nil {
		log.Printf("persist pending failed: %v", err)
	}
}

func (a *Agent) flushPending(ctx context.Context) error {
	backoff := time.Second
	for {
		a.mu.Lock()
		if len(a.pending) == 0 {
			a.mu.Unlock()
			return nil
		}
		next := a.pending[0]
		a.mu.Unlock()

		_, err := a.postAuthRawWithRetry(ctx, next.Path, next.Body, 5)
		if err != nil {
			if errors.Is(err, errUnauthorized) {
				a.triggerReauth()
			}
			if !sleepContext(ctx, backoffWithJitter(backoff)) {
				return err
			}
			backoff = nextBackoff(backoff)
			continue
		}
		backoff = time.Second

		a.mu.Lock()
		if len(a.pending) > 0 {
			a.pending = a.pending[1:]
			if err := a.persistPendingLocked(); err != nil {
				a.mu.Unlock()
				return err
			}
		}
		a.mu.Unlock()
	}
}

func (a *Agent) postAuthJSONWithRetry(ctx context.Context, path string, payload any, maxAttempts int) (apiEnvelope, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return apiEnvelope{}, err
	}
	return a.postAuthRawWithRetry(ctx, path, body, maxAttempts)
}

func (a *Agent) postAuthRawWithRetry(ctx context.Context, path string, body []byte, maxAttempts int) (apiEnvelope, error) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	backoff := time.Second
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		response, err := a.postAuthRaw(ctx, path, body)
		if err == nil {
			return response, nil
		}
		if errors.Is(err, errUnauthorized) {
			return apiEnvelope{}, err
		}
		if !isRetryableHTTPError(err) {
			return apiEnvelope{}, err
		}
		lastErr = err
		if attempt == maxAttempts {
			break
		}
		if !sleepContext(ctx, backoffWithJitter(backoff)) {
			return apiEnvelope{}, context.Canceled
		}
		backoff = nextBackoff(backoff)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("request retry exhausted")
	}
	return apiEnvelope{}, lastErr
}

func isRetryableHTTPError(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	var statusErr httpStatusError
	if errors.As(err, &statusErr) {
		if statusErr.StatusCode == http.StatusTooManyRequests {
			return true
		}
		return statusErr.StatusCode >= http.StatusInternalServerError
	}
	return true
}

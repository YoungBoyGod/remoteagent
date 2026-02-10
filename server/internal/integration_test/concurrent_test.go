package integration_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"luoyi2026/server/internal/api"
)

// TestConcurrent_MultipleAgentsRegister 多个 agent 并发注册
func TestConcurrent_MultipleAgentsRegister(t *testing.T) {
	r, _, mock, cleanup := newTestEnv(t)
	defer cleanup()

	agentCount := 5
	for i := 0; i < agentCount; i++ {
		mock.ExpectExec("insert into agents").
			WillReturnResult(sqlmock.NewResult(1, 1))
	}

	var wg sync.WaitGroup
	tokens := make([]string, agentCount)
	errs := make([]error, agentCount)

	for i := 0; i < agentCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			agentID := "agent-concurrent-" + string(rune('A'+idx))
			body := api.RegisterRequest{
				AgentID:    agentID,
				DeviceCode: "dev-" + agentID,
				Device:     api.DeviceInfo{Hostname: "h", OS: "linux", Arch: "amd64"},
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Register-Token", "test-token")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != 200 {
				errs[idx] = fmt.Errorf("agent %s register failed: %d", agentID, w.Code)
				return
			}
			env := parseEnvelope(t, w)
			data := env.Data.(map[string]any)
			tokens[idx] = data["token"].(string)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("agent %d: %v", i, err)
		}
		if tokens[i] == "" {
			t.Fatalf("agent %d: empty token", i)
		}
	}
}

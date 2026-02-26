package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestRoutingWriter_Write(t *testing.T) {
	t.Parallel()

	var all bytes.Buffer
	var srv bytes.Buffer
	var agt bytes.Buffer
	var errBuf bytes.Buffer

	w := &routingWriter{
		all:    &all,
		server: &srv,
		agent:  &agt,
		errW:   &errBuf,
	}

	cases := []struct {
		name       string
		line       string
		wantServer bool
		wantAgent  bool
		wantError  bool
	}{
		{
			name:       "local gin access goes to server",
			line:       "[GIN] 2026/02/20 - 10:00:00 | [L] 200 | 1ms | 127.0.0.1 | POST /api/v1/agent/heartbeat\n",
			wantServer: true,
		},
		{
			name:      "remote gin access goes to agent",
			line:      "[GIN] 2026/02/20 - 10:00:00 | [R] 200 | 1ms | 192.168.10.209 | POST /api/v1/agent/heartbeat\n",
			wantAgent: true,
		},
		{
			name:      "agent keyword runtime goes to agent",
			line:      "[GIN] 2026/02/20 - 10:00:00 | [R] 200 | 1ms | 192.168.10.209 | POST /api/v1/agents/a-1/poll\n",
			wantAgent: true,
		},
		{
			name:       "normal runtime goes to server",
			line:       "postgres connected: 127.0.0.1:35432/app_db\n",
			wantServer: true,
		},
		{
			name:      "failed goes to error",
			line:      "register failed: dial tcp ...\n",
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			all.Reset()
			srv.Reset()
			agt.Reset()
			errBuf.Reset()

			_, err := w.Write([]byte(tc.line))
			if err != nil {
				t.Fatalf("write failed: %v", err)
			}

			if !strings.Contains(all.String(), tc.line) {
				t.Fatalf("all.log should always contain line")
			}

			hasServer := strings.Contains(srv.String(), tc.line)
			hasAgent := strings.Contains(agt.String(), tc.line)
			hasError := strings.Contains(errBuf.String(), tc.line)

			if hasServer != tc.wantServer {
				t.Fatalf("server route mismatch: got=%v want=%v", hasServer, tc.wantServer)
			}
			if hasAgent != tc.wantAgent {
				t.Fatalf("agent route mismatch: got=%v want=%v", hasAgent, tc.wantAgent)
			}
			if hasError != tc.wantError {
				t.Fatalf("error route mismatch: got=%v want=%v", hasError, tc.wantError)
			}
		})
	}
}

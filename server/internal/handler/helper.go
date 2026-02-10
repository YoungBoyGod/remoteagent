package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"luoyi2026/server/internal/model"
)

func writeJSON(w http.ResponseWriter, status int, payload model.Envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, model.Envelope{Code: 405, Message: "method not allowed", RequestID: requestID()})
}

func writeBadRequest(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusBadRequest, model.Envelope{Code: 400, Message: message, RequestID: requestID()})
}

func writeAuthFailed(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, model.Envelope{Code: 401, Message: "unauthorized", RequestID: requestID()})
}

func writeServerError(w http.ResponseWriter) {
	writeJSON(w, http.StatusInternalServerError, model.Envelope{Code: 500, Message: "internal error", RequestID: requestID()})
}

func bearerToken(header string) (string, error) {
	if header == "" {
		return "", errors.New("empty authorization")
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errors.New("invalid authorization")
	}
	return parts[1], nil
}

func requestID() string {
	return "req-" + randHex(6)
}

func randHex(bytesLen int) string {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().Format("20060102150405")
	}
	return hex.EncodeToString(buf)
}

func toTaskSet(tasks []string) map[string]struct{} {
	set := make(map[string]struct{}, len(tasks))
	for _, taskID := range tasks {
		if taskID == "" {
			continue
		}
		set[taskID] = struct{}{}
	}
	return set
}

func isTaskStatus(status string) bool {
	return status == "running" || status == "success" || status == "failed" || status == "canceled"
}

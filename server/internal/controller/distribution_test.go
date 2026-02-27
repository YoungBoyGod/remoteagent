//go:build integration

package controller_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/config"
	"luoyi2026/server/internal/controller"
	"luoyi2026/server/internal/service"
)

// distColumns matches the RETURNING clause of InsertDistribution
var distColumns = []string{
	"id", "task_id", "file_name", "file_size", "sha256_original",
	"encryption_algo", "customer_name", "customer_email", "status",
	"release_notes", "scheduled_at", "created_at", "updated_at",
}

// distSelectColumns matches the SELECT columns in distSelectSQL
var distSelectColumns = []string{
	"id", "task_id", "file_name", "file_size",
	"encrypted_file_path", "sha256_original", "sha256_encrypted",
	"encryption_algo", "customer_name", "customer_email",
	"session_key_hash", "presigned_url", "url_expires_at",
	"status", "download_ip", "download_at",
	"release_notes", "scheduled_at", "created_at", "updated_at",
}

func setupDistRouter(svc *service.Service, cfg *config.Config) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")

	v1.POST("/distribute", controller.AdminAuth(cfg), controller.CreateDistributionHandler(svc))

	dist := v1.Group("/distributions", controller.AdminAuth(cfg))
	dist.GET("", controller.ListDistributionsHandler(svc))
	dist.GET("/s3-objects", controller.ListDistributionS3ObjectsHandler(svc))
	dist.GET("/:id", controller.GetDistributionHandler(svc))
	dist.PUT("/:id", controller.UpdateDistributionHandler(svc))
	dist.PATCH("/:id/status", controller.UpdateDistributionStatusHandler(svc))

	return r
}

func distCfg() *config.Config {
	return &config.Config{
		RegisterToken: "test-token",
		JWTTTL:        24 * time.Hour,
		PollTimeout:   5 * time.Second,
	}
}

func adminReq(method, url string, body interface{}) *http.Request {
	var buf *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewBuffer(b)
	} else {
		buf = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, url, buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	return req
}

// --- TestCreateDistribution ---

func TestCreateDistribution(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := distCfg()
	r := setupDistRouter(svc, cfg)

	now := time.Now()

	// InsertDistribution: INSERT ... RETURNING
	mock.ExpectQuery("INSERT INTO distributions").
		WillReturnRows(sqlmock.NewRows(distColumns).AddRow(
			1, "DIST-20260213-0001", "release.zip", 102400,
			"aabbccdd", "AES-256", "TestCorp", "test@example.com",
			"pending", "initial release", nil, now, now,
		))

	// CreateDistribution also calls CreateTask internally; if task creation fails,
	// distribution creation still succeeds.
	mock.ExpectQuery("(?i)insert into tasks").
		WillReturnError(fmt.Errorf("insert task failed"))

	body := api.DistributionCreateRequest{
		FileName:       "release.zip",
		FileSize:       102400,
		SHA256Original: "aabbccdd",
		EncryptionAlgo: "AES-256",
		CustomerName:   "TestCorp",
		CustomerEmail:  "test@example.com",
		ReleaseNotes:   "initial release",
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, adminReq(http.MethodPost, "/api/v1/distribute", body))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	env := parseEnvelope(t, w)
	if env.Code != 0 {
		t.Fatalf("expected code 0, got %d: %s", env.Code, env.Message)
	}

	data, ok := env.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", env.Data)
	}
	if data["task_id"] == nil || data["task_id"] == "" {
		t.Fatalf("expected non-empty task_id in response")
	}
}

// --- TestCreateDistributionMissingFields ---

func TestCreateDistributionMissingFields(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := distCfg()
	r := setupDistRouter(svc, cfg)

	tests := []struct {
		name string
		body api.DistributionCreateRequest
	}{
		{
			name: "missing file_name",
			body: api.DistributionCreateRequest{
				FileSize: 1024, SHA256Original: "abc",
				CustomerName: "C", CustomerEmail: "c@example.com",
			},
		},
		{
			name: "missing customer_email",
			body: api.DistributionCreateRequest{
				FileName: "f.zip", FileSize: 1024, SHA256Original: "abc",
				CustomerName: "C",
			},
		},
		{
			name: "missing sha256_original",
			body: api.DistributionCreateRequest{
				FileName: "f.zip", FileSize: 1024,
				CustomerName: "C", CustomerEmail: "c@example.com",
			},
		},
		{
			name: "invalid email format",
			body: api.DistributionCreateRequest{
				FileName: "f.zip", FileSize: 1024, SHA256Original: "abc",
				CustomerName: "C", CustomerEmail: "not-an-email",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, adminReq(http.MethodPost, "/api/v1/distribute", tc.body))

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d, body: %s", w.Code, w.Body.String())
			}
		})
	}
}

// --- TestListDistributions ---

func TestListDistributions(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := distCfg()
	r := setupDistRouter(svc, cfg)

	now := time.Now()

	// COUNT query
	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	// SELECT query
	rows := sqlmock.NewRows(distSelectColumns).
		AddRow(1, "DIST-001", "file1.zip", 1024,
			nil, "sha1", nil, "AES-256", "Corp1", "a@b.com",
			nil, nil, nil, "pending", nil, nil,
			nil, nil, now, now).
		AddRow(2, "DIST-002", "file2.zip", 2048,
			nil, "sha2", nil, "AES-256", "Corp2", "c@d.com",
			nil, nil, nil, "uploaded", nil, nil,
			"notes", nil, now, now)
	mock.ExpectQuery("SELECT d.id").WillReturnRows(rows)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, adminReq(http.MethodGet, "/api/v1/distributions?page=1&page_size=20", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	env := parseEnvelope(t, w)
	data, ok := env.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", env.Data)
	}

	total, _ := data["total"].(float64)
	if int(total) != 2 {
		t.Fatalf("expected total=2, got %v", data["total"])
	}

	items, ok := data["items"].([]interface{})
	if !ok || len(items) != 2 {
		t.Fatalf("expected 2 items, got %v", data["items"])
	}
}

// --- TestListDistributionsWithFilter ---

func TestListDistributionsWithFilter(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := distCfg()
	r := setupDistRouter(svc, cfg)

	now := time.Now()

	// COUNT with status filter
	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// SELECT with status filter
	rows := sqlmock.NewRows(distSelectColumns).
		AddRow(2, "DIST-002", "file2.zip", 2048,
			"/enc/file2.gpg", "sha2", "sha2enc", "AES-256", "Corp2", "c@d.com",
			nil, nil, nil, "uploaded", nil, nil,
			nil, nil, now, now)
	mock.ExpectQuery("SELECT d.id").WillReturnRows(rows)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, adminReq(http.MethodGet, "/api/v1/distributions?status=uploaded", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	env := parseEnvelope(t, w)
	data := env.Data.(map[string]interface{})

	total, _ := data["total"].(float64)
	if int(total) != 1 {
		t.Fatalf("expected total=1 for status=uploaded filter, got %v", data["total"])
	}

	items := data["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	item := items[0].(map[string]interface{})
	if item["status"] != "uploaded" {
		t.Fatalf("expected status=uploaded, got %v", item["status"])
	}
}

// --- TestGetDistributionDetail ---

func TestListDistributionS3ObjectsInvalidPrefix(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := distCfg()
	r := setupDistRouter(svc, cfg)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, adminReq(http.MethodGet, "/api/v1/distributions/s3-objects?prefix=../../etc", nil))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCreateDistributionWithS3SourceMissingKey(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := distCfg()
	r := setupDistRouter(svc, cfg)

	body := api.DistributionCreateRequest{
		FileName:    "placeholder.zip",
		SourceType:  "s3",
		CustomerName: "TestCorp",
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, adminReq(http.MethodPost, "/api/v1/distribute", body))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCreateDistributionWithS3Source(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := distCfg()
	r := setupDistRouter(svc, cfg)

	now := time.Now()

	mock.ExpectQuery("INSERT INTO distributions").
		WillReturnRows(sqlmock.NewRows(distColumns).AddRow(
			1, "DIST-20260213-0002", "releases/2026/release.zip", 0,
			"", "AES-256", "TestCorp", "test@example.com",
			"pending", "", nil, now, now,
		))
	mock.ExpectQuery("(?i)insert into tasks").
		WillReturnError(fmt.Errorf("insert task failed"))

	body := api.DistributionCreateRequest{
		FileName:      "placeholder.zip",
		SourceType:    "s3",
		S3Key:         "releases/2026/release.zip",
		CustomerName:  "TestCorp",
		CustomerEmail: "test@example.com",
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, adminReq(http.MethodPost, "/api/v1/distribute", body))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// --- TestGetDistributionDetail ---

func TestGetDistributionDetail(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := distCfg()
	r := setupDistRouter(svc, cfg)

	now := time.Now()

	mock.ExpectQuery("SELECT d.id").
		WillReturnRows(sqlmock.NewRows(distSelectColumns).AddRow(
			1, "DIST-001", "release.zip", 102400,
			"/enc/release.gpg", "sha_orig", "sha_enc", "AES-256",
			"TestCorp", "test@example.com",
			"keyhash", "https://s3.example.com/file", now,
			"uploaded", nil, nil,
			"v1.0 release", nil, now, now,
		))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, adminReq(http.MethodGet, "/api/v1/distributions/1", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	env := parseEnvelope(t, w)
	data, ok := env.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", env.Data)
	}

	if data["task_id"] != "DIST-001" {
		t.Fatalf("expected task_id=DIST-001, got %v", data["task_id"])
	}
	if data["file_name"] != "release.zip" {
		t.Fatalf("expected file_name=release.zip, got %v", data["file_name"])
	}
	if data["status"] != "uploaded" {
		t.Fatalf("expected status=uploaded, got %v", data["status"])
	}
}

func TestGetDistributionDetail_NotFound(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := distCfg()
	r := setupDistRouter(svc, cfg)

	mock.ExpectQuery("SELECT d.id").
		WillReturnRows(sqlmock.NewRows(distSelectColumns))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, adminReq(http.MethodGet, "/api/v1/distributions/999", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetDistributionDetail_InvalidID(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := distCfg()
	r := setupDistRouter(svc, cfg)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, adminReq(http.MethodGet, "/api/v1/distributions/abc", nil))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body: %s", w.Code, w.Body.String())
	}
}

// --- TestUpdateDistributionStatus ---

func TestUpdateDistributionStatus(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := distCfg()
	r := setupDistRouter(svc, cfg)

	// UpdateDistributionStatus uses a transaction:
	// 1. BEGIN
	// 2. SELECT status ... FOR UPDATE
	// 3. UPDATE ... SET status
	// 4. COMMIT
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM distributions").
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("pending"))
	mock.ExpectExec("UPDATE distributions SET").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	body := api.DistributionStatusRequest{
		Status: "encrypting",
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, adminReq(http.MethodPatch, "/api/v1/distributions/1/status", body))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUpdateDistributionStatus_InvalidTransition(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := distCfg()
	r := setupDistRouter(svc, cfg)

	// pending -> uploaded is not a valid transition (must go pending -> encrypting first)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM distributions").
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("pending"))
	mock.ExpectRollback()

	body := api.DistributionStatusRequest{
		Status: "uploaded",
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, adminReq(http.MethodPatch, "/api/v1/distributions/1/status", body))

	// Invalid transition returns 409 Conflict
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestUpdateDistributionStatus_NotFound(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := distCfg()
	r := setupDistRouter(svc, cfg)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM distributions").
		WillReturnError(fmt.Errorf("sql: no rows in result set"))
	mock.ExpectRollback()

	body := api.DistributionStatusRequest{
		Status: "encrypting",
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, adminReq(http.MethodPatch, "/api/v1/distributions/999/status", body))

	// Should return 404 or 500 depending on error handling
	if w.Code == http.StatusOK {
		t.Fatalf("expected non-200 for missing distribution, got 200")
	}
}

func TestUpdateDistributionStatus_MissingStatus(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := distCfg()
	r := setupDistRouter(svc, cfg)

	body := api.DistributionStatusRequest{}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, adminReq(http.MethodPatch, "/api/v1/distributions/1/status", body))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body: %s", w.Code, w.Body.String())
	}
}

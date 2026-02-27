package store_test

import (
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/store"
)

// --- GenDistTaskID ---

func TestGenDistTaskID_Format(t *testing.T) {
	id := store.GenDistTaskID()
	today := time.Now().Format("20060102")
	prefix := "DIST-" + today + "-"
	if len(id) != len(prefix)+8 {
		t.Fatalf("expected length %d, got %d: %s", len(prefix)+8, len(id), id)
	}
	if id[:len(prefix)] != prefix {
		t.Fatalf("expected prefix %s, got %s", prefix, id)
	}
}

func TestGenDistTaskID_Unique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id := store.GenDistTaskID()
		if seen[id] {
			t.Fatalf("duplicate task_id generated: %s", id)
		}
		seen[id] = true
	}
}

// --- ValidateDistTransition ---

func TestValidateDistTransition_ValidPaths(t *testing.T) {
	cases := []struct{ from, to string }{
		{"pending", "encrypting"},
		{"encrypting", "uploaded"},
		{"uploaded", "sent"},
		{"uploaded", "expired"},
		{"sent", "downloaded"},
		{"sent", "expired"},
	}
	for _, c := range cases {
		if err := store.ValidateDistTransition(c.from, c.to); err != nil {
			t.Errorf("expected valid transition %s -> %s, got error: %v", c.from, c.to, err)
		}
	}
}

func TestValidateDistTransition_InvalidPaths(t *testing.T) {
	cases := []struct{ from, to string }{
		{"pending", "uploaded"},
		{"pending", "sent"},
		{"encrypting", "sent"},
		{"uploaded", "downloaded"},
		{"downloaded", "sent"},
		{"expired", "pending"},
		{"downloaded", "expired"},
	}
	for _, c := range cases {
		if err := store.ValidateDistTransition(c.from, c.to); err == nil {
			t.Errorf("expected invalid transition %s -> %s, got nil", c.from, c.to)
		}
	}
}

func TestValidateDistTransition_UnknownStatus(t *testing.T) {
	err := store.ValidateDistTransition("nonexistent", "pending")
	if err == nil {
		t.Fatal("expected error for unknown status, got nil")
	}
}

// --- InsertDistribution ---

func TestInsertDistribution_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.DistributionCreateRequest{
		FileName:       "app-v1.0.tar.gz",
		FileSize:       1024000,
		SHA256Original: "abc123def456",
		CustomerName:   "TestCorp",
		CustomerEmail:  "test@example.com",
		ReleaseNotes:   "Initial release",
	}

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "task_id", "file_name", "file_size",
		"sha256_original", "encryption_algo",
		"customer_name", "customer_email", "status",
		"release_notes", "scheduled_at", "created_at", "updated_at",
	}).AddRow(
		1, "DIST-20260213-1234", "app-v1.0.tar.gz", int64(1024000),
		"abc123def456", "AES-256",
		"TestCorp", "test@example.com", "pending",
		"Initial release", nil, now, now,
	)

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO distributions")).
		WithArgs(
			sqlmock.AnyArg(), // task_id (generated)
			"app-v1.0.tar.gz", int64(1024000), "abc123def456", "AES-256",
			"TestCorp", "test@example.com", "Initial release", (*int64)(nil),
		).
		WillReturnRows(rows)

	item, err := store.InsertDistribution(db, req)
	if err != nil {
		t.Fatalf("InsertDistribution failed: %v", err)
	}
	if item.Status != "pending" {
		t.Fatalf("expected status pending, got %s", item.Status)
	}
	if item.EncryptionAlgo != "AES-256" {
		t.Fatalf("expected AES-256, got %s", item.EncryptionAlgo)
	}
	if item.FileName != "app-v1.0.tar.gz" {
		t.Fatalf("expected app-v1.0.tar.gz, got %s", item.FileName)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

func TestInsertDistribution_DefaultAlgo(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.DistributionCreateRequest{
		FileName:       "file.bin",
		FileSize:       100,
		SHA256Original: "aaa",
		CustomerName:   "C",
		CustomerEmail:  "c@c.com",
	}

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "task_id", "file_name", "file_size",
		"sha256_original", "encryption_algo",
		"customer_name", "customer_email", "status",
		"release_notes", "scheduled_at", "created_at", "updated_at",
	}).AddRow(1, "DIST-20260213-0001", "file.bin", int64(100),
		"aaa", "AES-256", "C", "c@c.com", "pending", "", nil, now, now)

	// EncryptionAlgo 为空时应默认 AES-256
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO distributions")).
		WithArgs(sqlmock.AnyArg(), "file.bin", int64(100), "aaa", "AES-256", "C", "c@c.com", "", (*int64)(nil)).
		WillReturnRows(rows)

	item, err := store.InsertDistribution(db, req)
	if err != nil {
		t.Fatalf("InsertDistribution failed: %v", err)
	}
	if item.EncryptionAlgo != "AES-256" {
		t.Fatalf("expected default AES-256, got %s", item.EncryptionAlgo)
	}
}

func TestInsertDistribution_WithScheduledAt(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	scheduledAt := time.Now().Add(30 * time.Minute).Unix()
	req := api.DistributionCreateRequest{
		FileName:       "scheduled-release.zip",
		FileSize:       4096,
		SHA256Original: "abc123",
		CustomerName:   "ScheduledCorp",
		CustomerEmail:  "scheduled@example.com",
		ReleaseNotes:   "scheduled release",
		ScheduledAt:    &scheduledAt,
	}

	now := time.Now()
	scheduledTime := time.Unix(scheduledAt, 0)
	rows := sqlmock.NewRows([]string{
		"id", "task_id", "file_name", "file_size",
		"sha256_original", "encryption_algo",
		"customer_name", "customer_email", "status",
		"release_notes", "scheduled_at", "created_at", "updated_at",
	}).AddRow(
		1, "DIST-20260227-0001", "scheduled-release.zip", int64(4096),
		"abc123", "AES-256",
		"ScheduledCorp", "scheduled@example.com", "pending",
		"scheduled release", scheduledTime, now, now,
	)

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO distributions")).
		WithArgs(
			sqlmock.AnyArg(), // task_id (generated)
			"scheduled-release.zip", int64(4096), "abc123", "AES-256",
			"ScheduledCorp", "scheduled@example.com", "scheduled release", scheduledAt,
		).
		WillReturnRows(rows)

	item, err := store.InsertDistribution(db, req)
	if err != nil {
		t.Fatalf("InsertDistribution failed: %v", err)
	}
	if item.ScheduledAt == nil {
		t.Fatal("expected scheduled_at in response, got nil")
	}
	if *item.ScheduledAt != scheduledAt {
		t.Fatalf("expected scheduled_at=%d, got %d", scheduledAt, *item.ScheduledAt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}


func TestGetDistributionByID_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "task_id", "file_name", "file_size",
		"encrypted_file_path", "sha256_original", "sha256_encrypted",
		"encryption_algo", "customer_name", "customer_email",
		"session_key_hash", "presigned_url", "url_expires_at",
		"status", "download_ip", "download_at",
		"release_notes", "scheduled_at", "created_at", "updated_at",
	}).AddRow(
		1, "DIST-20260213-1234", "app.tar.gz", int64(2048),
		nil, "sha-orig", nil,
		"AES-256", "Corp", "corp@test.com",
		nil, nil, nil,
		"pending", nil, nil,
		"notes", nil, now, now,
	)

	mock.ExpectQuery(regexp.QuoteMeta("FROM distributions d")).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	item, err := store.GetDistributionByID(db, 1)
	if err != nil {
		t.Fatalf("GetDistributionByID failed: %v", err)
	}
	if item == nil {
		t.Fatal("expected item, got nil")
	}
	if item.TaskID != "DIST-20260213-1234" {
		t.Fatalf("expected DIST-20260213-1234, got %s", item.TaskID)
	}
	if item.EncryptedFilePath != "" {
		t.Fatalf("expected empty encrypted_file_path, got %s", item.EncryptedFilePath)
	}
}

func TestGetDistributionByID_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"id", "task_id", "file_name", "file_size",
		"encrypted_file_path", "sha256_original", "sha256_encrypted",
		"encryption_algo", "customer_name", "customer_email",
		"session_key_hash", "presigned_url", "url_expires_at",
		"status", "download_ip", "download_at",
		"release_notes", "scheduled_at", "created_at", "updated_at",
	})

	mock.ExpectQuery(regexp.QuoteMeta("FROM distributions d")).
		WithArgs(int64(999)).
		WillReturnRows(rows)

	item, err := store.GetDistributionByID(db, 999)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if item != nil {
		t.Fatal("expected nil for not found")
	}
}

// --- GetDistributionByTaskID ---

func TestGetDistributionByTaskID_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "task_id", "file_name", "file_size",
		"encrypted_file_path", "sha256_original", "sha256_encrypted",
		"encryption_algo", "customer_name", "customer_email",
		"session_key_hash", "presigned_url", "url_expires_at",
		"status", "download_ip", "download_at",
		"release_notes", "scheduled_at", "created_at", "updated_at",
	}).AddRow(
		5, "DIST-20260213-5555", "data.zip", int64(4096),
		"/enc/data.zip.enc", "sha-o", "sha-e",
		"AES-256", "Client", "client@test.com",
		"keyhash", "https://s3.example.com/file", now,
		"uploaded", nil, nil,
		"v2 release", nil, now, now,
	)

	mock.ExpectQuery(regexp.QuoteMeta("FROM distributions d")).
		WithArgs("DIST-20260213-5555").
		WillReturnRows(rows)

	item, err := store.GetDistributionByTaskID(db, "DIST-20260213-5555")
	if err != nil {
		t.Fatalf("GetDistributionByTaskID failed: %v", err)
	}
	if item == nil {
		t.Fatal("expected item, got nil")
	}
	if item.EncryptedFilePath != "/enc/data.zip.enc" {
		t.Fatalf("expected /enc/data.zip.enc, got %s", item.EncryptedFilePath)
	}
	if item.URLExpiresAt == nil {
		t.Fatal("expected url_expires_at to be set")
	}
}

// --- UpdateDistribution ---

func TestUpdateDistribution_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.DistributionUpdateRequest{
		EncryptedFilePath: "/enc/file.enc",
		SHA256Encrypted:   "sha-enc-123",
		SessionKeyHash:    "keyhash-456",
	}

	mock.ExpectExec(regexp.QuoteMeta("UPDATE distributions SET")).
		WithArgs("/enc/file.enc", "sha-enc-123", "keyhash-456", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = store.UpdateDistribution(db, 1, req)
	if err != nil {
		t.Fatalf("UpdateDistribution failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

func TestUpdateDistribution_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.DistributionUpdateRequest{
		EncryptedFilePath: "/enc/file.enc",
	}

	mock.ExpectExec(regexp.QuoteMeta("UPDATE distributions SET")).
		WithArgs("/enc/file.enc", int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = store.UpdateDistribution(db, 999, req)
	if err == nil {
		t.Fatal("expected error for not found, got nil")
	}
	if err.Error() != "distribution not found" {
		t.Fatalf("expected 'distribution not found', got: %v", err)
	}
}

func TestUpdateDistribution_EmptyRequest(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.DistributionUpdateRequest{}
	err = store.UpdateDistribution(db, 1, req)
	if err != nil {
		t.Fatalf("expected nil for empty update, got: %v", err)
	}
}

// --- UpdateDistributionStatus ---

func TestUpdateDistributionStatus_ValidTransition(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM distributions WHERE id = $1 FOR UPDATE")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("pending"))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE distributions SET")).
		WithArgs(int64(1), "encrypting").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	req := api.DistributionStatusRequest{Status: "encrypting"}
	err = store.UpdateDistributionStatus(db, 1, req)
	if err != nil {
		t.Fatalf("UpdateDistributionStatus failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

func TestUpdateDistributionStatus_InvalidTransition(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM distributions WHERE id = $1 FOR UPDATE")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("pending"))
	mock.ExpectRollback()

	req := api.DistributionStatusRequest{Status: "downloaded"}
	err = store.UpdateDistributionStatus(db, 1, req)
	if err == nil {
		t.Fatal("expected error for invalid transition, got nil")
	}
}

func TestUpdateDistributionStatus_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM distributions WHERE id = $1 FOR UPDATE")).
		WithArgs(int64(999)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}))
	mock.ExpectRollback()

	req := api.DistributionStatusRequest{Status: "encrypting"}
	err = store.UpdateDistributionStatus(db, 999, req)
	if err == nil {
		t.Fatal("expected error for not found, got nil")
	}
	if err.Error() != "distribution not found" {
		t.Fatalf("expected 'distribution not found', got: %v", err)
	}
}

func TestUpdateDistributionStatus_Downloaded_WithIP(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM distributions WHERE id = $1 FOR UPDATE")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("sent"))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE distributions SET")).
		WithArgs(int64(1), "downloaded", "192.168.1.100").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	req := api.DistributionStatusRequest{
		Status:     "downloaded",
		DownloadIP: "192.168.1.100",
	}
	err = store.UpdateDistributionStatus(db, 1, req)
	if err != nil {
		t.Fatalf("UpdateDistributionStatus failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// --- ListDistributions ---

func TestListDistributions_Defaults(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(regexp.QuoteMeta("FROM distributions d")).
		WithArgs(20, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "task_id", "file_name", "file_size",
			"encrypted_file_path", "sha256_original", "sha256_encrypted",
			"encryption_algo", "customer_name", "customer_email",
			"session_key_hash", "presigned_url", "url_expires_at",
			"status", "download_ip", "download_at",
			"release_notes", "scheduled_at", "created_at", "updated_at",
		}))

	req := api.DistributionListRequest{}
	resp, err := store.ListDistributions(db, req)
	if err != nil {
		t.Fatalf("ListDistributions failed: %v", err)
	}
	if resp.Page != 1 {
		t.Fatalf("expected page 1, got %d", resp.Page)
	}
	if resp.PageSize != 20 {
		t.Fatalf("expected page_size 20, got %d", resp.PageSize)
	}
	if resp.Total != 0 {
		t.Fatalf("expected total 0, got %d", resp.Total)
	}
	if len(resp.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(resp.Items))
	}
}

func TestListDueScheduledDistributions(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now()
	scheduled := now.Add(-5 * time.Minute)
	rows := sqlmock.NewRows([]string{
		"id", "task_id", "file_name", "file_size",
		"encrypted_file_path", "sha256_original", "sha256_encrypted",
		"encryption_algo", "customer_name", "customer_email",
		"session_key_hash", "presigned_url", "url_expires_at",
		"status", "download_ip", "download_at",
		"release_notes", "scheduled_at", "created_at", "updated_at",
	}).AddRow(
		7, "DIST-20260227-7777", "scheduled.bin", int64(8192),
		nil, "sha7", nil,
		"AES-256", "DelayCorp", "delay@example.com",
		nil, nil, nil,
		"pending", nil, nil,
		"scheduled", scheduled, now, now,
	)

	mock.ExpectQuery(regexp.QuoteMeta("FROM distributions d")).
		WithArgs(20).
		WillReturnRows(rows)

	items, err := store.ListDueScheduledDistributions(db, 20)
	if err != nil {
		t.Fatalf("ListDueScheduledDistributions failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ScheduledAt == nil {
		t.Fatal("expected scheduled_at, got nil")
	}
	if items[0].TaskID != "DIST-20260227-7777" {
		t.Fatalf("expected task_id DIST-20260227-7777, got %s", items[0].TaskID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}


func TestClearDistributionScheduledAt(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE distributions")).
		WithArgs("DIST-20260227-7777").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = store.ClearDistributionScheduledAt(db, "DIST-20260227-7777")
	if err != nil {
		t.Fatalf("ClearDistributionScheduledAt failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}


func TestListDistributions_PageSizeCap(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(regexp.QuoteMeta("FROM distributions d")).
		WithArgs(100, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "task_id", "file_name", "file_size",
			"encrypted_file_path", "sha256_original", "sha256_encrypted",
			"encryption_algo", "customer_name", "customer_email",
			"session_key_hash", "presigned_url", "url_expires_at",
			"status", "download_ip", "download_at",
			"release_notes", "scheduled_at", "created_at", "updated_at",
		}))

	req := api.DistributionListRequest{PageSize: 500}
	resp, err := store.ListDistributions(db, req)
	if err != nil {
		t.Fatalf("ListDistributions failed: %v", err)
	}
	if resp.PageSize != 100 {
		t.Fatalf("expected page_size capped at 100, got %d", resp.PageSize)
	}
}

func TestInsertDistribution_DBClosed(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	db.Close()

	req := api.DistributionCreateRequest{
		FileName: "f", FileSize: 1, SHA256Original: "s",
		CustomerName: "c", CustomerEmail: "e@e.com",
	}
	_, err = store.InsertDistribution(db, req)
	if err == nil {
		t.Fatal("expected error on closed db, got nil")
	}
}

func TestGetDistributionByID_DBClosed(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	db.Close()

	_, err = store.GetDistributionByID(db, 1)
	if err == nil {
		t.Fatal("expected error on closed db, got nil")
	}
}

func TestUpdateDistributionStatus_DBClosed(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	db.Close()

	req := api.DistributionStatusRequest{Status: "encrypting"}
	err = store.UpdateDistributionStatus(db, 1, req)
	if err == nil {
		t.Fatal("expected error on closed db, got nil")
	}
}

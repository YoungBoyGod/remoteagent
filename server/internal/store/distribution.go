package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"strings"
	"time"

	"luoyi2026/server/internal/api"
)

// Distribution 状态常量
const (
	DistStatusPending    = "pending"
	DistStatusEncrypting = "encrypting"
	DistStatusUploaded   = "uploaded"
	DistStatusSent       = "sent"
	DistStatusDownloaded = "downloaded"
	DistStatusExpired    = "expired"
)

// validDistTransitions 状态机：定义合法的状态转换
var validDistTransitions = map[string][]string{
	DistStatusPending:    {DistStatusEncrypting},
	DistStatusEncrypting: {DistStatusUploaded},
	DistStatusUploaded:   {DistStatusSent, DistStatusExpired},
	DistStatusSent:       {DistStatusDownloaded, DistStatusExpired},
	DistStatusDownloaded: {},
	DistStatusExpired:    {},
}

// ValidateDistTransition 校验状态转换是否合法
func ValidateDistTransition(from, to string) error {
	targets, ok := validDistTransitions[from]
	if !ok {
		return fmt.Errorf("unknown status: %s", from)
	}
	for _, t := range targets {
		if t == to {
			return nil
		}
	}
	return fmt.Errorf("invalid status transition: %s -> %s", from, to)
}

// GenDistTaskID 生成分发任务编号：DIST-YYYYMMDD-XXXX
func GenDistTaskID() string {
	date := time.Now().Format("20060102")
	n, _ := rand.Int(rand.Reader, big.NewInt(10000))
	return fmt.Sprintf("DIST-%s-%04d", date, n.Int64())
}

// InsertDistribution 创建分发记录
func InsertDistribution(db *sql.DB, req api.DistributionCreateRequest) (*api.DistributionItem, error) {
	taskID := GenDistTaskID()
	algo := req.EncryptionAlgo
	if algo == "" {
		algo = "AES-256"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var item api.DistributionItem
	var createdAt, updatedAt time.Time

	err := db.QueryRowContext(ctx,
		`INSERT INTO distributions (task_id, file_name, file_size, sha256_original, encryption_algo,
		    customer_name, customer_email, release_notes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, task_id, file_name, file_size, sha256_original, encryption_algo,
		    customer_name, customer_email, status, release_notes, created_at, updated_at`,
		taskID, req.FileName, req.FileSize, req.SHA256Original, algo,
		req.CustomerName, req.CustomerEmail, req.ReleaseNotes,
	).Scan(
		&item.ID, &item.TaskID, &item.FileName, &item.FileSize,
		&item.SHA256Original, &item.EncryptionAlgo,
		&item.CustomerName, &item.CustomerEmail, &item.Status,
		&item.ReleaseNotes, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	item.CreatedAt = createdAt.Unix()
	item.UpdatedAt = updatedAt.Unix()
	return &item, nil
}

// GetDistributionByID 按主键查询
func GetDistributionByID(db *sql.DB, id int64) (*api.DistributionItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return scanDistribution(db.QueryRowContext(ctx,
		distSelectSQL+" WHERE d.id = $1", id))
}

// GetDistributionByTaskID 按 task_id 查询
func GetDistributionByTaskID(db *sql.DB, taskID string) (*api.DistributionItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return scanDistribution(db.QueryRowContext(ctx,
		distSelectSQL+" WHERE d.task_id = $1", taskID))
}

// ListDistributions 分页查询分发列表，支持筛选和排序
func ListDistributions(db *sql.DB, req api.DistributionListRequest) (*api.DistributionListResponse, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	where := []string{"1=1"}
	args := []any{}
	idx := 1

	if req.Status != "" {
		statuses := strings.Split(req.Status, ",")
		if len(statuses) == 1 {
			where = append(where, fmt.Sprintf("d.status = $%d", idx))
			args = append(args, req.Status)
			idx++
		} else {
			placeholders := make([]string, len(statuses))
			for i, s := range statuses {
				placeholders[i] = fmt.Sprintf("$%d", idx)
				args = append(args, strings.TrimSpace(s))
				idx++
			}
			where = append(where, fmt.Sprintf("d.status IN (%s)", strings.Join(placeholders, ",")))
		}
	}
	if req.Search != "" {
		where = append(where, fmt.Sprintf(
			"(d.task_id ILIKE $%d OR d.file_name ILIKE $%d OR d.customer_name ILIKE $%d OR d.customer_email ILIKE $%d)",
			idx, idx, idx, idx))
		args = append(args, "%"+req.Search+"%")
		idx++
	}

	whereClause := strings.Join(where, " AND ")

	// 排序
	orderBy := "d.created_at DESC"
	allowedSorts := map[string]string{
		"created_at": "d.created_at",
		"file_name":  "d.file_name",
		"status":     "d.status",
		"file_size":  "d.file_size",
	}
	if col, ok := allowedSorts[req.SortBy]; ok {
		dir := "DESC"
		if strings.EqualFold(req.SortDir, "asc") {
			dir = "ASC"
		}
		orderBy = col + " " + dir
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 查询总数
	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM distributions d WHERE "+whereClause, countArgs...,
	).Scan(&total)
	if err != nil {
		return nil, err
	}

	// 查询分页数据
	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)
	query := fmt.Sprintf(
		`%s WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d`,
		distSelectSQL, whereClause, orderBy, idx, idx+1)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []api.DistributionItem{}
	for rows.Next() {
		item, err := scanDistributionRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &api.DistributionListResponse{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Items:    items,
	}, nil
}

// UpdateDistribution 更新分发记录字段
func UpdateDistribution(db *sql.DB, id int64, req api.DistributionUpdateRequest) error {
	sets := []string{}
	args := []any{}
	idx := 1

	if req.EncryptedFilePath != "" {
		sets = append(sets, fmt.Sprintf("encrypted_file_path = $%d", idx))
		args = append(args, req.EncryptedFilePath)
		idx++
	}
	if req.SHA256Encrypted != "" {
		sets = append(sets, fmt.Sprintf("sha256_encrypted = $%d", idx))
		args = append(args, req.SHA256Encrypted)
		idx++
	}
	if req.SessionKeyHash != "" {
		sets = append(sets, fmt.Sprintf("session_key_hash = $%d", idx))
		args = append(args, req.SessionKeyHash)
		idx++
	}
	if req.PresignedURL != "" {
		sets = append(sets, fmt.Sprintf("presigned_url = $%d", idx))
		args = append(args, req.PresignedURL)
		idx++
	}
	if req.URLExpiresAt != nil {
		sets = append(sets, fmt.Sprintf("url_expires_at = to_timestamp($%d)", idx))
		args = append(args, *req.URLExpiresAt)
		idx++
	}
	if req.ReleaseNotes != "" {
		sets = append(sets, fmt.Sprintf("release_notes = $%d", idx))
		args = append(args, req.ReleaseNotes)
		idx++
	}
	if req.CustomerName != "" {
		sets = append(sets, fmt.Sprintf("customer_name = $%d", idx))
		args = append(args, req.CustomerName)
		idx++
	}
	if req.CustomerEmail != "" {
		sets = append(sets, fmt.Sprintf("customer_email = $%d", idx))
		args = append(args, req.CustomerEmail)
		idx++
	}

	if len(sets) == 0 {
		return nil
	}

	sets = append(sets, "updated_at = now()")
	args = append(args, id)

	query := fmt.Sprintf("UPDATE distributions SET %s WHERE id = $%d",
		strings.Join(sets, ", "), idx)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("distribution not found")
	}
	return nil
}

// UpdateDistributionStatus 更新分发状态（含状态机校验）
func UpdateDistributionStatus(db *sql.DB, id int64, req api.DistributionStatusRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 查询当前状态
	var currentStatus string
	err = tx.QueryRowContext(ctx,
		"SELECT status FROM distributions WHERE id = $1 FOR UPDATE", id,
	).Scan(&currentStatus)
	if err == sql.ErrNoRows {
		return fmt.Errorf("distribution not found")
	}
	if err != nil {
		return err
	}

	// 校验状态转换
	if err := ValidateDistTransition(currentStatus, req.Status); err != nil {
		return err
	}

	// 构建更新
	sets := []string{
		"status = $2",
		"updated_at = now()",
	}
	args := []any{id, req.Status}
	idx := 3

	// downloaded 状态自动记录下载时间和 IP
	if req.Status == DistStatusDownloaded {
		sets = append(sets, "download_at = now()")
		if req.DownloadIP != "" {
			sets = append(sets, fmt.Sprintf("download_ip = $%d", idx))
			args = append(args, req.DownloadIP)
		}
	}

	query := fmt.Sprintf("UPDATE distributions SET %s WHERE id = $1",
		strings.Join(sets, ", "))

	_, err = tx.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ---- 内部辅助 ----

const distSelectSQL = `SELECT d.id, d.task_id, d.file_name, d.file_size,
	d.encrypted_file_path, d.sha256_original, d.sha256_encrypted,
	d.encryption_algo, d.customer_name, d.customer_email,
	d.session_key_hash, d.presigned_url, d.url_expires_at,
	d.status, d.download_ip, d.download_at,
	d.release_notes, d.created_at, d.updated_at
FROM distributions d`

// scanDistribution 从单行查询扫描 DistributionItem
func scanDistribution(row *sql.Row) (*api.DistributionItem, error) {
	var item api.DistributionItem
	var encPath, sha256Enc, sessionKeyHash, presignedURL, downloadIP, releaseNotes sql.NullString
	var urlExpiresAt, downloadAt sql.NullTime
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&item.ID, &item.TaskID, &item.FileName, &item.FileSize,
		&encPath, &item.SHA256Original, &sha256Enc,
		&item.EncryptionAlgo, &item.CustomerName, &item.CustomerEmail,
		&sessionKeyHash, &presignedURL, &urlExpiresAt,
		&item.Status, &downloadIP, &downloadAt,
		&releaseNotes, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	fillDistNullables(&item, encPath, sha256Enc, sessionKeyHash, presignedURL,
		downloadIP, releaseNotes, urlExpiresAt, downloadAt)
	item.CreatedAt = createdAt.Unix()
	item.UpdatedAt = updatedAt.Unix()
	return &item, nil
}

// scanner 接口用于统一 *sql.Row 和 *sql.Rows 的 Scan 方法
type scanner interface {
	Scan(dest ...any) error
}

// scanDistributionRow 从结果集行扫描 DistributionItem
func scanDistributionRow(rows scanner) (*api.DistributionItem, error) {
	var item api.DistributionItem
	var encPath, sha256Enc, sessionKeyHash, presignedURL, downloadIP, releaseNotes sql.NullString
	var urlExpiresAt, downloadAt sql.NullTime
	var createdAt, updatedAt time.Time

	err := rows.Scan(
		&item.ID, &item.TaskID, &item.FileName, &item.FileSize,
		&encPath, &item.SHA256Original, &sha256Enc,
		&item.EncryptionAlgo, &item.CustomerName, &item.CustomerEmail,
		&sessionKeyHash, &presignedURL, &urlExpiresAt,
		&item.Status, &downloadIP, &downloadAt,
		&releaseNotes, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	fillDistNullables(&item, encPath, sha256Enc, sessionKeyHash, presignedURL,
		downloadIP, releaseNotes, urlExpiresAt, downloadAt)
	item.CreatedAt = createdAt.Unix()
	item.UpdatedAt = updatedAt.Unix()
	return &item, nil
}

func fillDistNullables(item *api.DistributionItem,
	encPath, sha256Enc, sessionKeyHash, presignedURL, downloadIP, releaseNotes sql.NullString,
	urlExpiresAt, downloadAt sql.NullTime) {

	if encPath.Valid {
		item.EncryptedFilePath = encPath.String
	}
	if sha256Enc.Valid {
		item.SHA256Encrypted = sha256Enc.String
	}
	if sessionKeyHash.Valid {
		item.SessionKeyHash = sessionKeyHash.String
	}
	if presignedURL.Valid {
		item.PresignedURL = presignedURL.String
	}
	if downloadIP.Valid {
		item.DownloadIP = downloadIP.String
	}
	if releaseNotes.Valid {
		item.ReleaseNotes = releaseNotes.String
	}
	if urlExpiresAt.Valid {
		ts := urlExpiresAt.Time.Unix()
		item.URLExpiresAt = &ts
	}
	if downloadAt.Valid {
		ts := downloadAt.Time.Unix()
		item.DownloadAt = &ts
	}
}

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"tg-search/internal/model"
)

type FileRepository struct {
	db *sql.DB
}

func NewFileRepository(db *sql.DB) *FileRepository {
	return &FileRepository{db: db}
}

func (r *FileRepository) SaveBatch(ctx context.Context, messageID int64, files []model.File) ([]model.File, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	out, err := r.SaveBatchTx(ctx, tx, messageID, files)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *FileRepository) SaveBatchTx(ctx context.Context, tx *sql.Tx, messageID int64, files []model.File) ([]model.File, error) {
	out := make([]model.File, 0, len(files))
	now := time.Now().UTC()
	for _, file := range files {
		file.MessageID = messageID
		file.Extension = normalizeExtension(file.Extension, file.FileName)
		if file.Category == "" {
			file.Category = fileCategory(file)
		}
		err := tx.QueryRowContext(ctx, `
INSERT INTO telegram_files
  (message_id, file_name, extension, mime_type, size_bytes, category, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(message_id, file_name, size_bytes) DO UPDATE SET
  extension = excluded.extension,
  mime_type = excluded.mime_type,
  category = excluded.category,
  updated_at = excluded.updated_at
RETURNING id, created_at, updated_at`,
			file.MessageID, file.FileName, file.Extension, file.MimeType, file.SizeBytes, file.Category, now, now,
		).Scan(&file.ID, &file.CreatedAt, &file.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("save file %s: %w", file.FileName, err)
		}
		out = append(out, file)
	}
	return out, nil
}

func (r *FileRepository) ReplaceForMessageTx(ctx context.Context, tx *sql.Tx, messageID int64, files []model.File) ([]model.File, error) {
	if _, err := tx.ExecContext(ctx, `DELETE FROM telegram_files WHERE message_id = ?`, messageID); err != nil {
		return nil, fmt.Errorf("delete old files: %w", err)
	}
	return r.SaveBatchTx(ctx, tx, messageID, files)
}

func (r *FileRepository) FindByMessageID(ctx context.Context, messageID int64) ([]model.File, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, message_id, file_name, extension, mime_type, size_bytes, category, created_at, updated_at
FROM telegram_files
WHERE message_id = ?
ORDER BY id`, messageID)
	if err != nil {
		return nil, fmt.Errorf("find files: %w", err)
	}
	defer rows.Close()

	var out []model.File
	for rows.Next() {
		file, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, file)
	}
	return out, rows.Err()
}

func (r *FileRepository) Search(ctx context.Context, params FileSearchParams) ([]model.FileResult, error) {
	limit := clampLimit(params.Limit, 50)
	where, args := fileSearchWhere(params)
	args = append(args, limit, params.Offset)

	query := `
SELECT f.id, f.message_id, f.file_name, f.extension, f.mime_type, f.size_bytes, f.category, f.created_at, f.updated_at,
       mc.text, m.date, m.account_id, m.channel_id, c.title, m.telegram_message_id
FROM telegram_files f
JOIN telegram_messages m ON m.id = f.message_id
JOIN telegram_message_contents mc ON mc.message_id = m.id
JOIN telegram_channels c ON c.id = m.channel_id
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY ` + dateOrderBy(params.Sort, "m.date", "f.id") + `
LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search files: %w", err)
	}
	defer rows.Close()

	var out []model.FileResult
	for rows.Next() {
		var item model.FileResult
		if err := rows.Scan(&item.ID, &item.MessageID, &item.FileName, &item.Extension, &item.MimeType, &item.SizeBytes, &item.Category, &item.CreatedAt, &item.UpdatedAt, &item.MessageText, &item.MessageDate, &item.AccountID, &item.ChannelID, &item.ChannelTitle, &item.TelegramMessageID); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *FileRepository) CountSearch(ctx context.Context, params FileSearchParams) (int, error) {
	where, args := fileSearchWhere(params)
	query := `
SELECT count(*)
FROM telegram_files f
JOIN telegram_messages m ON m.id = f.message_id
JOIN telegram_message_contents mc ON mc.message_id = m.id
WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count search files: %w", err)
	}
	return total, nil
}

func (r *FileRepository) CountResources(ctx context.Context, params FileSearchParams) (int, error) {
	where, args := fileSearchWhere(params)
	query := `
SELECT count(*)
FROM telegram_files f
JOIN telegram_messages m ON m.id = f.message_id
JOIN telegram_message_contents mc ON mc.message_id = m.id
WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count resource files: %w", err)
	}
	return total, nil
}

func fileSearchWhere(params FileSearchParams) ([]string, []any) {
	where := []string{`m.deleted = 0`}
	args := []any{}
	if params.Query != "" {
		like := "%" + params.Query + "%"
		where = append(where, `(f.file_name LIKE ? OR f.mime_type LIKE ? OR mc.text LIKE ?)`)
		args = append(args, like, like, like)
	}
	if params.Category != "" {
		where = append(where, `f.category = ?`)
		args = append(args, params.Category)
	}
	if params.Extension != "" {
		extension := normalizeExtension(params.Extension, "")
		where = append(where, `f.extension = ?`)
		args = append(args, extension)
	}
	if params.AccountID > 0 {
		where = append(where, `m.account_id = ?`)
		args = append(args, params.AccountID)
	}
	if params.ChannelID > 0 {
		where = append(where, `m.channel_id = ?`)
		args = append(args, params.ChannelID)
	}
	if params.DateFrom != nil {
		where = append(where, `m.date >= ?`)
		args = append(args, *params.DateFrom)
	}
	if params.DateTo != nil {
		where = append(where, `m.date < ?`)
		args = append(args, *params.DateTo)
	}
	return where, args
}

func scanFile(row interface {
	Scan(...any) error
}) (model.File, error) {
	var file model.File
	err := row.Scan(&file.ID, &file.MessageID, &file.FileName, &file.Extension, &file.MimeType, &file.SizeBytes, &file.Category, &file.CreatedAt, &file.UpdatedAt)
	if err != nil {
		return model.File{}, err
	}
	return file, nil
}

func normalizeExtension(extension string, fileName string) string {
	extension = strings.ToLower(strings.TrimSpace(extension))
	if extension != "" {
		if !strings.HasPrefix(extension, ".") {
			return "." + extension
		}
		return extension
	}
	return strings.ToLower(filepath.Ext(fileName))
}

func fileCategory(file model.File) string {
	ext := strings.ToLower(file.Extension)
	mimeType := strings.ToLower(file.MimeType)
	switch {
	case ext == ".mp4" || ext == ".mkv" || ext == ".avi" || strings.HasPrefix(mimeType, "video/"):
		return "video"
	case ext == ".pdf" || ext == ".epub" || ext == ".mobi":
		return "ebook"
	case ext == ".zip" || ext == ".rar" || ext == ".7z":
		return "archive"
	case ext == ".iso" || ext == ".exe" || ext == ".apk" || strings.Contains(mimeType, "application/"):
		return "software"
	default:
		return "file"
	}
}

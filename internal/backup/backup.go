package backup

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func SQLite(ctx context.Context, conn *sql.DB, backupDir string) (string, error) {
	if backupDir == "" {
		return "", fmt.Errorf("backup directory is required")
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}
	path := filepath.Join(backupDir, "telegram-"+time.Now().UTC().Format("20060102-150405.000000000")+".db")
	if _, err := conn.ExecContext(ctx, `VACUUM INTO ?`, path); err != nil {
		return "", fmt.Errorf("backup sqlite database: %w", err)
	}
	return path, nil
}

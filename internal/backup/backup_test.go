package backup

import (
	"context"
	"path/filepath"
	"testing"

	"tg-provider/internal/db"
	"tg-provider/internal/model"
	"tg-provider/internal/repository"
)

func TestSQLiteBackupWritesConsistentDatabaseCopy(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	accounts := repository.NewAccountRepository(conn)
	if _, err := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline}); err != nil {
		t.Fatalf("save account: %v", err)
	}

	path, err := SQLite(ctx, conn, t.TempDir())
	if err != nil {
		t.Fatalf("SQLite returned error: %v", err)
	}

	backupConn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer backupConn.Close()
	counts, err := repository.NewStatusRepository(backupConn).Counts(ctx)
	if err != nil {
		t.Fatalf("backup counts: %v", err)
	}
	if counts.Accounts != 1 {
		t.Fatalf("backup accounts = %d, want 1", counts.Accounts)
	}
}

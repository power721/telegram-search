package session

import (
	"fmt"
	"os"
	"path/filepath"
)

type Manager struct {
	dir string
}

func NewManager(dir string) *Manager {
	return &Manager{dir: dir}
}

func (m *Manager) PathForAccount(accountID int64) string {
	return filepath.Join(m.dir, fmt.Sprintf("account-%d.session.json", accountID))
}

func (m *Manager) RemoveForAccount(accountID int64) error {
	err := os.Remove(m.PathForAccount(accountID))
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return fmt.Errorf("remove account session %d: %w", accountID, err)
}

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

func (m *Manager) PathForTemporary(name string) string {
	return filepath.Join(m.dir, name+".session.json")
}

func (m *Manager) MoveTemporaryToAccount(tempPath string, accountID int64) (string, error) {
	finalPath := m.PathForAccount(accountID)
	if err := os.Rename(tempPath, finalPath); err != nil {
		return "", fmt.Errorf("move temporary session to account %d: %w", accountID, err)
	}
	return finalPath, nil
}

func (m *Manager) RemoveForAccount(accountID int64) error {
	err := os.Remove(m.PathForAccount(accountID))
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return fmt.Errorf("remove account session %d: %w", accountID, err)
}

func (m *Manager) RemovePath(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return fmt.Errorf("remove session path %q: %w", path, err)
}

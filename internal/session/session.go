package session

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
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
	if err := waitForFile(tempPath, 2*time.Second); err != nil {
		return "", fmt.Errorf("move temporary session to account %d: %w", accountID, err)
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o700); err != nil {
		return "", fmt.Errorf("prepare account session directory: %w", err)
	}
	if err := os.Remove(finalPath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("replace account session %d: %w", accountID, err)
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return "", fmt.Errorf("move temporary session to account %d: %w", accountID, err)
	}
	return finalPath, nil
}

func waitForFile(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		if info, err := os.Stat(path); err == nil {
			if !info.IsDir() {
				return nil
			}
			return fmt.Errorf("temporary session path %q is a directory", path)
		} else {
			lastErr = err
			if !os.IsNotExist(err) {
				return err
			}
		}
		if time.Now().After(deadline) {
			return lastErr
		}
		time.Sleep(25 * time.Millisecond)
	}
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

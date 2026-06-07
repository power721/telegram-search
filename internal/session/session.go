package session

import (
	"fmt"
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

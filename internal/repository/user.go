package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"tg-search/internal/model"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user model.User) (int64, error) {
	now := time.Now().UTC()
	if user.Role == "" {
		user.Role = model.UserRoleAdmin
	}
	res, err := r.db.ExecContext(ctx, `
INSERT INTO users
  (username, password_hash, role, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?)`,
		user.Username, user.PasswordHash, user.Role, now, now)
	if err != nil {
		return 0, fmt.Errorf("create user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("create user id: %w", err)
	}
	return id, nil
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (model.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, `
SELECT id, username, password_hash, role, last_login_at, created_at, updated_at
FROM users WHERE username = ?`, username))
}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (model.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, `
SELECT id, username, password_hash, role, last_login_at, created_at, updated_at
FROM users WHERE id = ?`, id))
}

func (r *UserRepository) UpdateCredentials(ctx context.Context, id int64, username string, passwordHash string) (model.User, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
UPDATE users
SET username = ?, password_hash = ?, updated_at = ?
WHERE id = ?`,
		username, passwordHash, now, id)
	if err != nil {
		return model.User{}, fmt.Errorf("update user credentials: %w", err)
	}
	if err := requireRows(res, "user not found"); err != nil {
		return model.User{}, err
	}
	return r.FindByID(ctx, id)
}

func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return count, nil
}

func (r *UserRepository) UpdateLastLogin(ctx context.Context, id int64, at time.Time) error {
	res, err := r.db.ExecContext(ctx, `
UPDATE users SET last_login_at = ?, updated_at = ? WHERE id = ?`, at, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update user last login: %w", err)
	}
	return requireRows(res, "user not found")
}

func scanUser(row interface {
	Scan(...any) error
}) (model.User, error) {
	var user model.User
	var lastLogin sql.NullTime
	err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &lastLogin, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, sql.ErrNoRows
	}
	if err != nil {
		return model.User{}, err
	}
	if lastLogin.Valid {
		user.LastLoginAt = &lastLogin.Time
	}
	return user, nil
}

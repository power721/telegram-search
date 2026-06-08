package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"tg-search/internal/model"
)

type AccountRepository struct {
	db *sql.DB
}

func NewAccountRepository(db *sql.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

func (r *AccountRepository) Save(ctx context.Context, account model.Account) (int64, error) {
	now := time.Now().UTC()
	if account.Status == "" {
		account.Status = model.AccountStatusNew
	}
	var id int64
	err := r.db.QueryRowContext(ctx, `
INSERT INTO telegram_accounts
  (phone, telegram_user_id, first_name, last_name, username, status, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(phone) DO UPDATE SET
  telegram_user_id = excluded.telegram_user_id,
  first_name = excluded.first_name,
  last_name = excluded.last_name,
  username = excluded.username,
  status = excluded.status,
  updated_at = excluded.updated_at
RETURNING id`,
		account.Phone, account.TelegramUserID, account.FirstName, account.LastName, account.Username, account.Status, now, now,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("save account: %w", err)
	}
	return id, nil
}

func (r *AccountRepository) Update(ctx context.Context, account model.Account) error {
	res, err := r.db.ExecContext(ctx, `
UPDATE telegram_accounts
SET phone = ?, telegram_user_id = ?, first_name = ?, last_name = ?, username = ?, status = ?, updated_at = ?
WHERE id = ?`,
		account.Phone, account.TelegramUserID, account.FirstName, account.LastName, account.Username, account.Status, time.Now().UTC(), account.ID)
	if err != nil {
		return fmt.Errorf("update account: %w", err)
	}
	return requireRows(res, "account not found")
}

func (r *AccountRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	res, err := r.db.ExecContext(ctx, `
UPDATE telegram_accounts
SET status = ?, updated_at = ?
WHERE id = ?`, status, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update account status: %w", err)
	}
	return requireRows(res, "account not found")
}

func (r *AccountRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM telegram_accounts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	return requireRows(res, "account not found")
}

func (r *AccountRepository) FindByID(ctx context.Context, id int64) (model.Account, error) {
	return scanAccount(r.db.QueryRowContext(ctx, `
SELECT id, phone, telegram_user_id, first_name, last_name, username, status, created_at, updated_at
FROM telegram_accounts WHERE id = ?`, id))
}

func (r *AccountRepository) FindByPhone(ctx context.Context, phone string) (model.Account, error) {
	return scanAccount(r.db.QueryRowContext(ctx, `
SELECT id, phone, telegram_user_id, first_name, last_name, username, status, created_at, updated_at
FROM telegram_accounts WHERE phone = ?`, phone))
}

func (r *AccountRepository) FindAll(ctx context.Context) ([]model.Account, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, phone, telegram_user_id, first_name, last_name, username, status, created_at, updated_at
FROM telegram_accounts ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("find accounts: %w", err)
	}
	defer rows.Close()

	var out []model.Account
	for rows.Next() {
		account, err := scanAccountRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, account)
	}
	return out, rows.Err()
}

func scanAccount(row interface {
	Scan(...any) error
}) (model.Account, error) {
	account, err := scanAccountRows(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Account{}, sql.ErrNoRows
	}
	return account, err
}

func scanAccountRows(row interface {
	Scan(...any) error
}) (model.Account, error) {
	var account model.Account
	err := row.Scan(&account.ID, &account.Phone, &account.TelegramUserID, &account.FirstName, &account.LastName, &account.Username, &account.Status, &account.CreatedAt, &account.UpdatedAt)
	if err != nil {
		return model.Account{}, err
	}
	return account, nil
}

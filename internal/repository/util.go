package repository

import (
	"database/sql"
	"errors"
	"fmt"
)

func requireRows(res sql.Result, notFound string) error {
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w: %s", sql.ErrNoRows, notFound)
	}
	return nil
}

func isNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

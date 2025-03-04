package db

import (
	"fmt"
)

type DBError struct {
	Operation string
	Err       error
}

func (e *DBError) Error() string {
	return fmt.Sprintf("database operation '%s' failed: %v", e.Operation, e.Err)
}

func wrapError(op string, err error) error {
	if err == nil {
		return nil
	}
	return &DBError{
		Operation: op,
		Err:       err,
	}
}

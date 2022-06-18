package app

import (
	"fmt"
)

type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("Internal server error: %s", e.Message)
}

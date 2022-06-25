package app

import (
	"fmt"
)

type appError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *appError) Error() string {
	return fmt.Sprintf("Internal server error: %s", e.Message)
}

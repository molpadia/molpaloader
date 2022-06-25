package app

type appError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Returns an error message.
func (e *appError) Error() string {
	return e.Message
}

// Compares the given error matches the message.
func (e *appError) Is(err error) bool {
	return e.Message == err.Error()
}

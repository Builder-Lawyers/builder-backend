package errs

import "fmt"

type PermissionsError struct {
	Err error
}

func (t PermissionsError) Error() string {
	return fmt.Sprintf("error in permissions: %v", t.Err)
}

type RetryableError struct {
	Err error
}

func (t RetryableError) Error() string {
	return fmt.Sprintf("retryable error: %v", t.Err)
}

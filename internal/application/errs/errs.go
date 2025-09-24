package errs

import "fmt"

type PermissionsError struct {
	Err error
}

func (t PermissionsError) Error() string {
	return fmt.Sprintf("error in permissions: %v", t.Err)
}

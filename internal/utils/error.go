package utils

import "fmt"

func MakeError(format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	panic(err)
	return err
}

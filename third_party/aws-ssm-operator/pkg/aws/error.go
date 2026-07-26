package aws

import (
	"fmt"
	"strings"
)

type SSMError struct {
	Err             error
	ParameterErrors []ParameterError
}

type ParameterError struct {
	Name string
	Err  error
}

func NewSSMError(msg string) *SSMError {
	return &SSMError{Err: fmt.Errorf(msg)}
}

func (e *SSMError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	var b strings.Builder

	for _, err := range e.ParameterErrors {
		b.WriteString(err.Error())
	}
	return b.String()
}

func (e *ParameterError) Error() string {
	return fmt.Sprintf("%s %s", e.Name, e.Err)
}

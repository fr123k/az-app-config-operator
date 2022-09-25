package azure

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSSMError(t *testing.T) {
	err := SSMError{Err: fmt.Errorf("This is an expected general error!")}

	assert.Equal(t, "This is an expected general error!", err.Error())
}

func TestParameterError(t *testing.T) {
	err := SSMError{
		ParameterErrors: []ParameterError{{
			Name: "/ssm/param1",
			Err:  fmt.Errorf("The param %s was not found.", "/ssm/param1"),
		}, {
			Name: "/ssm/param2",
			Err:  fmt.Errorf("The param %s was not found.", "/ssm/param2"),
		},
		},
	}

	assert.Equal(t, "/ssm/param1 The param /ssm/param1 was not found./ssm/param2 The param /ssm/param2 was not found.", err.Error())
}

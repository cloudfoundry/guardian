package testhelpers

import (
	"fmt"
	"reflect"

	"github.com/onsi/gomega/types"
	errorspkg "github.com/pkg/errors"
)

func BeErrorType(expected interface{}) types.GomegaMatcher {
	return &beErrorTypeMatcher{
		expected: expected,
	}
}

type beErrorTypeMatcher struct {
	expected interface{}
}

func (matcher *beErrorTypeMatcher) Match(actual interface{}) (success bool, err error) {
	if actual == nil {
		return false, nil
	}

	_, ok := matcher.expected.(error)
	if !ok {
		return false, fmt.Errorf("BeErrorType matcher expects an error")
	}

	actualErr, ok := actual.(error)
	if !ok {
		return false, fmt.Errorf("BeErrorType matcher expects an error")
	}

	cause := errorspkg.Cause(actualErr)

	beautifulVar := reflect.PtrTo(reflect.TypeOf(matcher.expected))
	return reflect.TypeOf(cause) == beautifulVar, nil
}

func (matcher *beErrorTypeMatcher) FailureMessage(actual interface{}) (message string) {
	if actual == nil {
		return fmt.Sprintf("Expected error, got nil")
	}

	actualErr, _ := actual.(error)
	return fmt.Sprintf("Expected error\n\t%s\nto be of type\n\t%s", actualErr.Error(), reflect.TypeOf(matcher.expected))
}

func (matcher *beErrorTypeMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	actualErr, _ := actual.(error)
	return fmt.Sprintf("Expected error\n\t%s\nnot to be of type\n\t%s", actualErr.Error(), reflect.TypeOf(matcher.expected))
}

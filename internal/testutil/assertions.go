package testutil

import (
	"fmt"
	"os"
	"reflect"
	"strings"
)

// AssertContains asserts that the actual string contains the expected substring.
// It marks the calling test as a helper and fails immediately if the assertion fails.
func AssertContains(t TestingT, actual, expected string) {
	t.Helper()

	if !strings.Contains(actual, expected) {
		t.Fatalf("assertion failed: expected string to contain %q\nActual: %s", expected, actual)
	}
}

// AssertNotContains asserts that the actual string does NOT contain the unexpected substring.
// It marks the calling test as a helper and fails immediately if the assertion fails.
func AssertNotContains(t TestingT, actual, unexpected string) {
	t.Helper()

	if strings.Contains(actual, unexpected) {
		t.Fatalf("assertion failed: expected string NOT to contain %q\nActual: %s", unexpected, actual)
	}
}

// AssertEqual asserts that two values are equal using reflect.DeepEqual.
// It marks the calling test as a helper and fails immediately if the assertion fails.
func AssertEqual(t TestingT, expected, actual interface{}) {
	t.Helper()

	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("assertion failed: values not equal\nExpected: %v\nActual:   %v", expected, actual)
	}
}

// AssertNotEqual asserts that two values are NOT equal using reflect.DeepEqual.
// It marks the calling test as a helper and fails immediately if the assertion fails.
func AssertNotEqual(t TestingT, unexpected, actual interface{}) {
	t.Helper()

	if reflect.DeepEqual(unexpected, actual) {
		t.Fatalf("assertion failed: values should not be equal\nUnexpected: %v\nActual:     %v", unexpected, actual)
	}
}

// AssertError asserts that the error is not nil.
// It marks the calling test as a helper and fails immediately if err is nil.
// Optional msgAndArgs can be provided for additional context.
func AssertError(t TestingT, err error, msgAndArgs ...interface{}) {
	t.Helper()

	if err == nil {
		msg := formatMessage("expected error, got nil", msgAndArgs...)
		t.Fatalf("assertion failed: %s", msg)
	}
}

// AssertNoError asserts that the error is nil.
// It marks the calling test as a helper and fails immediately if err is not nil.
// Optional msgAndArgs can be provided for additional context.
func AssertNoError(t TestingT, err error, msgAndArgs ...interface{}) {
	t.Helper()

	if err != nil {
		msg := formatMessage("expected no error", msgAndArgs...)
		t.Fatalf("assertion failed: %s, got: %v", msg, err)
	}
}

// AssertFileExists asserts that a file exists at the given path.
// It marks the calling test as a helper and fails immediately if the file doesn't exist.
func AssertFileExists(t TestingT, path string) {
	t.Helper()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("assertion failed: file does not exist: %s", path)
	} else if err != nil {
		t.Fatalf("assertion failed: error checking file existence: %v\nPath: %s", err, path)
	}
}

// AssertFileNotExists asserts that a file does NOT exist at the given path.
// It marks the calling test as a helper and fails immediately if the file exists.
func AssertFileNotExists(t TestingT, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Fatalf("assertion failed: file should not exist: %s", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("assertion failed: error checking file non-existence: %v\nPath: %s", err, path)
	}
}

// formatMessage formats an optional message with arguments.
func formatMessage(defaultMsg string, msgAndArgs ...interface{}) string {
	if len(msgAndArgs) == 0 {
		return defaultMsg
	}

	if len(msgAndArgs) == 1 {
		msg, ok := msgAndArgs[0].(string)
		if ok {
			return msg
		}
		return fmt.Sprintf("%v", msgAndArgs[0])
	}

	format, ok := msgAndArgs[0].(string)
	if !ok {
		return defaultMsg
	}

	return fmt.Sprintf(format, msgAndArgs[1:]...)
}

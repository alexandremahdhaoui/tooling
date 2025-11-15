//go:build unit

package testutil

import (
	"errors"
	"os"
	"testing"
)

func TestAssertContains_Success(t *testing.T) {
	// Should not fail
	AssertContains(t, "hello world", "hello")
	AssertContains(t, "hello world", "world")
	AssertContains(t, "hello world", "lo wo")
}

func TestAssertContains_Failure(t *testing.T) {
	mockT := &mockTestingT{t: t}
	AssertContains(mockT, "hello world", "goodbye")

	if !mockT.failed {
		t.Fatal("expected AssertContains to fail")
	}
}

func TestAssertNotContains_Success(t *testing.T) {
	// Should not fail
	AssertNotContains(t, "hello world", "goodbye")
	AssertNotContains(t, "hello world", "HELLO")
}

func TestAssertNotContains_Failure(t *testing.T) {
	mockT := &mockTestingT{t: t}
	AssertNotContains(mockT, "hello world", "hello")

	if !mockT.failed {
		t.Fatal("expected AssertNotContains to fail")
	}
}

func TestAssertEqual_Success(t *testing.T) {
	// Should not fail
	AssertEqual(t, 42, 42)
	AssertEqual(t, "hello", "hello")
	AssertEqual(t, []int{1, 2, 3}, []int{1, 2, 3})
	AssertEqual(t, map[string]int{"a": 1}, map[string]int{"a": 1})
}

func TestAssertEqual_Failure(t *testing.T) {
	mockT := &mockTestingT{t: t}
	AssertEqual(mockT, 42, 43)

	if !mockT.failed {
		t.Fatal("expected AssertEqual to fail")
	}
}

func TestAssertNotEqual_Success(t *testing.T) {
	// Should not fail
	AssertNotEqual(t, 42, 43)
	AssertNotEqual(t, "hello", "world")
	AssertNotEqual(t, []int{1, 2, 3}, []int{1, 2, 4})
}

func TestAssertNotEqual_Failure(t *testing.T) {
	mockT := &mockTestingT{t: t}
	AssertNotEqual(mockT, 42, 42)

	if !mockT.failed {
		t.Fatal("expected AssertNotEqual to fail")
	}
}

func TestAssertError_Success(t *testing.T) {
	// Should not fail
	AssertError(t, errors.New("some error"))
	AssertError(t, errors.New("another error"), "custom message")
}

func TestAssertError_Failure(t *testing.T) {
	mockT := &mockTestingT{t: t}
	AssertError(mockT, nil)

	if !mockT.failed {
		t.Fatal("expected AssertError to fail when error is nil")
	}
}

func TestAssertNoError_Success(t *testing.T) {
	// Should not fail
	AssertNoError(t, nil)
	AssertNoError(t, nil, "custom message")
}

func TestAssertNoError_Failure(t *testing.T) {
	mockT := &mockTestingT{t: t}
	AssertNoError(mockT, errors.New("some error"))

	if !mockT.failed {
		t.Fatal("expected AssertNoError to fail when error is not nil")
	}
}

func TestAssertFileExists_Success(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "testutil-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Should not fail
	AssertFileExists(t, tmpFile.Name())
}

func TestAssertFileExists_Failure(t *testing.T) {
	mockT := &mockTestingT{t: t}
	AssertFileExists(mockT, "/nonexistent/file/path/12345")

	if !mockT.failed {
		t.Fatal("expected AssertFileExists to fail for non-existent file")
	}
}

func TestAssertFileNotExists_Success(t *testing.T) {
	// Should not fail
	AssertFileNotExists(t, "/nonexistent/file/path/12345")
}

func TestAssertFileNotExists_Failure(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "testutil-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	mockT := &mockTestingT{t: t}
	AssertFileNotExists(mockT, tmpFile.Name())

	if !mockT.failed {
		t.Fatal("expected AssertFileNotExists to fail for existing file")
	}
}

func TestFormatMessage_NoArgs(t *testing.T) {
	msg := formatMessage("default")
	if msg != "default" {
		t.Fatalf("expected 'default', got %q", msg)
	}
}

func TestFormatMessage_SingleString(t *testing.T) {
	msg := formatMessage("default", "custom message")
	if msg != "custom message" {
		t.Fatalf("expected 'custom message', got %q", msg)
	}
}

func TestFormatMessage_Format(t *testing.T) {
	msg := formatMessage("default", "value is %d", 42)
	if msg != "value is 42" {
		t.Fatalf("expected 'value is 42', got %q", msg)
	}
}

func TestFormatMessage_SingleNonString(t *testing.T) {
	msg := formatMessage("default", 42)
	if msg != "42" {
		t.Fatalf("expected '42', got %q", msg)
	}
}

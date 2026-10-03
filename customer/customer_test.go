package http

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lib/pq"
)

func TestValidName(t *testing.T) {
	name := "test USer"
	want := "Test User"

	got := normalizeName(name)
	if got != want {
		t.Fatalf("normalizeName(%q) = %q, want %q", name, got, want)
	}
}

func TestSingleWordNameReturnsError(t *testing.T) {
	err := validateName("test")

	if !errors.Is(err, errNameTwoWords) {
		t.Fatalf("validateName(%q) error = %v, want %v", "test", err, errNameTwoWords)
	}
}

func TestNameIsRequired(t *testing.T) {
	err := validateRequired("", "test@example.com")

	if !errors.Is(err, errNameEmailRequired) {
		t.Fatalf("validateRequired(%q, %q) error = %v, want %v", "", "test@example.com", err, errNameEmailRequired)
	}
}

func TestEmailIsRequired(t *testing.T) {
	err := validateRequired("Test User", "")

	if !errors.Is(err, errNameEmailRequired) {
		t.Fatalf("validateRequired(%q, %q) error = %v, want %v", "Test User", "", err, errNameEmailRequired)
	}
}

func TestNameAndEmailMissing(t *testing.T) {
	err := validateRequired("", "")

	if !errors.Is(err, errNameEmailRequired) {
		t.Fatalf("validateRequired(%q, %q) error = %v, want %v", "", "", err, errNameEmailRequired)
	}
}

func TestNameAndEmailArePresent(t *testing.T) {
	err := validateRequired("Test User", "test@example.com")

	if err != nil {
		t.Fatalf("validateRequired(%q, %q) error = %v, want nil", "Test User", "test@example.com", err)
	}
}

func TestDuplicateEmailReturnsEmailExists(t *testing.T) {
	dbErr := &pq.Error{Code: "23505"}

	err := mapCreateError(dbErr)

	if !errors.Is(err, errEmailExists) {
		t.Fatalf("mapCreateError(unique violation) = %v, want %v", err, errEmailExists)
	}
}

func TestWrappedDuplicateEmailReturnsEmailExists(t *testing.T) {
	dbErr := fmt.Errorf("create customer: %w", &pq.Error{Code: "23505"})

	err := mapCreateError(dbErr)

	if !errors.Is(err, errEmailExists) {
		t.Fatalf("mapCreateError(wrapped unique violation) = %v, want %v", err, errEmailExists)
	}
}

func TestOtherDBErrorReturnsInternal(t *testing.T) {
	dbErr := errors.New("connection refused")

	err := mapCreateError(dbErr)

	if !errors.Is(err, errInternal) {
		t.Fatalf("mapCreateError(%v) = %v, want %v", dbErr, err, errInternal)
	}
}

func TestOtherPostgresCodeReturnsInternal(t *testing.T) {
	dbErr := &pq.Error{Code: "23502"}

	err := mapCreateError(dbErr)

	if !errors.Is(err, errInternal) {
		t.Fatalf("mapCreateError(code 23502) = %v, want %v", err, errInternal)
	}
}

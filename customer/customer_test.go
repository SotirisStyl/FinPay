package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/labstack/echo"
	"github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestValidName(t *testing.T) {
	name := "test USer"
	want := "Test User"

	got := normalizeName(name)
	if got != want {
		t.Fatalf("normalizeName(%q) = %q, want %q", name, got, want)
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

func newTestHandlers(t *testing.T) (handlers, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	return handlers{DB: db}, mock
}

func postCustomer(t *testing.T, h handlers, body string) *httptest.ResponseRecorder {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/customers", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.create(c); err != nil {
		t.Fatalf("create returned error: %v", err)
	}
	return rec
}

func errorBody(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v (body=%q)", err, rec.Body.String())
	}
	return resp["error"]
}

func assertMockMet(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet DB expectations: %v", err)
	}
}

func TestCreate_Success(t *testing.T) {
	h, mock := newTestHandlers(t)
	mock.ExpectExec(`INSERT INTO "customers"`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	rec := postCustomer(t, h, `{"name":"test USer","email":"test@example.com"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var got Customer
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not valid customer JSON: %v", err)
	}
	if got.Name != "Test User" {
		t.Errorf("name = %q, want %q", got.Name, "Test User")
	}
	if got.Email != "test@example.com" {
		t.Errorf("email = %q, want %q", got.Email, "test@example.com")
	}
	assertMockMet(t, mock)
}

func TestCreate_MissingName(t *testing.T) {
	h, mock := newTestHandlers(t)

	rec := postCustomer(t, h, `{"email":"test@example.com"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := errorBody(t, rec); got != errNameEmailRequired.Error() {
		t.Errorf("error = %q, want %q", got, errNameEmailRequired.Error())
	}
	assertMockMet(t, mock)
}

func TestCreate_MissingEmail(t *testing.T) {
	h, mock := newTestHandlers(t)

	rec := postCustomer(t, h, `{"name":"Test User"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := errorBody(t, rec); got != errNameEmailRequired.Error() {
		t.Errorf("error = %q, want %q", got, errNameEmailRequired.Error())
	}
	assertMockMet(t, mock)
}

func TestCreate_InvalidJSON(t *testing.T) {
	h, mock := newTestHandlers(t)

	rec := postCustomer(t, h, `{not json`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := errorBody(t, rec); got != "invalid input" {
		t.Errorf("error = %q, want %q", got, "invalid input")
	}
	assertMockMet(t, mock)
}

func TestCreate_DuplicateEmail(t *testing.T) {
	h, mock := newTestHandlers(t)
	mock.ExpectExec(`INSERT INTO "customers"`).
		WillReturnError(&pq.Error{Code: "23505"})

	rec := postCustomer(t, h, `{"name":"Test User","email":"dupe@example.com"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	if got := errorBody(t, rec); got != errEmailExists.Error() {
		t.Errorf("error = %q, want %q", got, errEmailExists.Error())
	}
	assertMockMet(t, mock)
}

func TestCreate_DatabaseFailure(t *testing.T) {
	h, mock := newTestHandlers(t)
	mock.ExpectExec(`INSERT INTO "customers"`).
		WillReturnError(errors.New("connection refused"))

	rec := postCustomer(t, h, `{"name":"Test User","email":"test@example.com"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := errorBody(t, rec); got != errInternal.Error() {
		t.Errorf("error = %q, want %q", got, errInternal.Error())
	}
	assertMockMet(t, mock)
}

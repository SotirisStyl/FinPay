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
	mock.ExpectQuery(`INSERT INTO "customers"`).
		WithArgs("Test User", "test@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("11111111-1111-4111-8111-111111111111"))

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
	if got.ID != "11111111-1111-4111-8111-111111111111" {
		t.Errorf("id = %q, want database-generated UUID", got.ID)
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
	mock.ExpectQuery(`INSERT INTO "customers"`).
		WithArgs("Test User", "dupe@example.com").
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
	mock.ExpectQuery(`INSERT INTO "customers"`).
		WithArgs("Test User", "test@example.com").
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

func findCustomer(t *testing.T, h handlers, id string) *httptest.ResponseRecorder {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/customers/"+id, nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/customers/:id")
	c.SetParamNames("id")
	c.SetParamValues(id)

	if err := h.get(c); err != nil {
		t.Fatalf("get returned error: %v", err)
	}
	return rec
}

func patchCustomer(t *testing.T, h handlers, id, body string) *httptest.ResponseRecorder {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/customers/"+id, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/customers/:id")
	c.SetParamNames("id")
	c.SetParamValues(id)

	if err := h.update(c); err != nil {
		t.Fatalf("update returned error: %v", err)
	}
	return rec
}

func TestGet_CustomerFound(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"

	h, mock := newTestHandlers(t)
	mock.ExpectQuery(`SELECT \* FROM "customers"`).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).
			AddRow(id, "Test User", "test@example.com"))

	rec := findCustomer(t, h, id)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got Customer
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not valid customer JSON: %v", err)
	}
	if got.ID != id {
		t.Errorf("id = %q, want %q", got.ID, id)
	}
	if got.Name != "Test User" {
		t.Errorf("name = %q, want %q", got.Name, "Test User")
	}
	if got.Email != "test@example.com" {
		t.Errorf("email = %q, want %q", got.Email, "test@example.com")
	}
	assertMockMet(t, mock)
}

func TestGet_MissingUuid(t *testing.T) {
	const id = ""

	h, mock := newTestHandlers(t)

	rec := findCustomer(t, h, id)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if got := errorBody(t, rec); got != "invalid uuid" {
		t.Errorf("error = %q, want %q", got, "invalid uuid")
	}
	assertMockMet(t, mock)
}

func TestGet_InvalidUuid(t *testing.T) {
	const id = "test"

	h, mock := newTestHandlers(t)

	rec := findCustomer(t, h, id)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if got := errorBody(t, rec); got != "invalid uuid" {
		t.Errorf("error = %q, want %q", got, "invalid uuid")
	}
	assertMockMet(t, mock)
}

func TestGet_CustomerNotFound(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"

	h, mock := newTestHandlers(t)
	mock.ExpectQuery(`SELECT \* FROM "customers"`).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}))

	rec := findCustomer(t, h, id)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := errorBody(t, rec); got != errCustomerNotFound.Error() {
		t.Errorf("error = %q, want %q", got, errCustomerNotFound.Error())
	}
	assertMockMet(t, mock)
}

func TestGet_DatabaseFailure(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"

	h, mock := newTestHandlers(t)
	mock.ExpectQuery(`SELECT \* FROM "customers"`).
		WithArgs(id, 1).
		WillReturnError(errors.New("connection refused"))

	rec := findCustomer(t, h, id)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := errorBody(t, rec); got != errInternal.Error() {
		t.Errorf("error = %q, want %q", got, errInternal.Error())
	}
	assertMockMet(t, mock)
}

func TestUpdate_DuplicateEmailReturnsConflict(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"

	h, mock := newTestHandlers(t)
	mock.ExpectQuery(`SELECT \* FROM "customers"`).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).
			AddRow(id, "Mary Smith", "old@example.com"))
	mock.ExpectExec(`UPDATE "customers" SET "email"=\$1 WHERE "customers"\."id" = \$2`).
		WithArgs("mary@gmail.com", id).
		WillReturnError(&pq.Error{Code: "23505"})

	rec := patchCustomer(t, h, id, `{"email":"mary@gmail.com"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	if got := errorBody(t, rec); got != errEmailExists.Error() {
		t.Errorf("error = %q, want %q", got, errEmailExists.Error())
	}
	assertMockMet(t, mock)
}

func TestUpdate_EmailOnlyPreservesOtherFields(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"

	h, mock := newTestHandlers(t)
	mock.ExpectQuery(`SELECT \* FROM "customers"`).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).
			AddRow(id, "Mary Smith", "old@example.com"))
	mock.ExpectExec(`UPDATE "customers" SET "email"=\$1 WHERE "customers"\."id" = \$2`).
		WithArgs("new@example.com", id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	rec := patchCustomer(t, h, id, `{"email":"new@example.com"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got Customer
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not valid customer JSON: %v", err)
	}
	if got.Name != "Mary Smith" {
		t.Errorf("name = %q, want existing name %q", got.Name, "Mary Smith")
	}
	if got.Email != "new@example.com" {
		t.Errorf("email = %q, want %q", got.Email, "new@example.com")
	}
	assertMockMet(t, mock)
}

func TestUpdate_NameAndEmail(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"

	h, mock := newTestHandlers(t)
	mock.ExpectQuery(`SELECT \* FROM "customers"`).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).
			AddRow(id, "Mary Smith", "old@example.com"))
	mock.ExpectExec(`UPDATE "customers" SET "email"=\$1,"name"=\$2 WHERE "customers"\."id" = \$3`).
		WithArgs("new@example.com", "Mary Jones", id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	rec := patchCustomer(t, h, id, `{"name":"mary jones","email":"new@example.com"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got Customer
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not valid customer JSON: %v", err)
	}
	if got.Name != "Mary Jones" {
		t.Errorf("name = %q, want %q", got.Name, "Mary Jones")
	}
	if got.Email != "new@example.com" {
		t.Errorf("email = %q, want %q", got.Email, "new@example.com")
	}
	assertMockMet(t, mock)
}

func TestUpdate_EmptyNameReturnsBadRequest(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"

	h, mock := newTestHandlers(t)
	mock.ExpectQuery(`SELECT \* FROM "customers"`).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).
			AddRow(id, "Mary Smith", "mary@example.com"))

	rec := patchCustomer(t, h, id, `{"name":""}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := errorBody(t, rec); got != errNameEmailRequired.Error() {
		t.Errorf("error = %q, want %q", got, errNameEmailRequired.Error())
	}
	assertMockMet(t, mock)
}

func TestUpdate_EmptyEmailReturnsBadRequest(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"

	h, mock := newTestHandlers(t)
	mock.ExpectQuery(`SELECT \* FROM "customers"`).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).
			AddRow(id, "Mary Smith", "mary@example.com"))

	rec := patchCustomer(t, h, id, `{"email":""}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := errorBody(t, rec); got != errNameEmailRequired.Error() {
		t.Errorf("error = %q, want %q", got, errNameEmailRequired.Error())
	}
	assertMockMet(t, mock)
}

func TestUpdate_NameOnlyPreservesExistingEmail(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"

	h, mock := newTestHandlers(t)
	mock.ExpectQuery(`SELECT \* FROM "customers"`).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).
			AddRow(id, "Mary Smith", "mary@example.com"))
	mock.ExpectExec(`UPDATE "customers" SET "name"=\$1 WHERE "customers"\."id" = \$2`).
		WithArgs("Alice", id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	rec := patchCustomer(t, h, id, `{"name":"Alice"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got Customer
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not valid customer JSON: %v", err)
	}
	if got.Name != "Alice" {
		t.Errorf("name = %q, want %q", got.Name, "Alice")
	}
	if got.Email != "mary@example.com" {
		t.Errorf("email = %q, want existing email %q", got.Email, "mary@example.com")
	}
	assertMockMet(t, mock)
}

func TestUpdate_CurrentEmailReturnsOK(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"

	h, mock := newTestHandlers(t)
	mock.ExpectQuery(`SELECT \* FROM "customers"`).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).
			AddRow(id, "Mary Smith", "mary@example.com"))
	mock.ExpectExec(`UPDATE "customers" SET "email"=\$1 WHERE "customers"\."id" = \$2`).
		WithArgs("mary@example.com", id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	rec := patchCustomer(t, h, id, `{"email":"mary@example.com"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got Customer
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not valid customer JSON: %v", err)
	}
	if got.Email != "mary@example.com" {
		t.Errorf("email = %q, want %q", got.Email, "mary@example.com")
	}
	assertMockMet(t, mock)
}

func TestUpdate_InvalidUUID(t *testing.T) {
	h, mock := newTestHandlers(t)

	rec := patchCustomer(t, h, "not-a-uuid", `{"email":"new@example.com"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := errorBody(t, rec); got != errInvalidUuid.Error() {
		t.Errorf("error = %q, want %q", got, errInvalidUuid.Error())
	}
	assertMockMet(t, mock)
}

func TestUpdate_InvalidJSON(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"

	h, mock := newTestHandlers(t)

	rec := patchCustomer(t, h, id, `{invalid json`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := errorBody(t, rec); got != "invalid input" {
		t.Errorf("error = %q, want %q", got, "invalid input")
	}
	assertMockMet(t, mock)
}

func TestUpdate_CustomerNotFound(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"

	h, mock := newTestHandlers(t)
	mock.ExpectQuery(`SELECT \* FROM "customers"`).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}))

	rec := patchCustomer(t, h, id, `{"email":"new@example.com"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := errorBody(t, rec); got != errCustomerNotFound.Error() {
		t.Errorf("error = %q, want %q", got, errCustomerNotFound.Error())
	}
	assertMockMet(t, mock)
}

func TestUpdate_LookupDatabaseFailure(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"

	h, mock := newTestHandlers(t)
	mock.ExpectQuery(`SELECT \* FROM "customers"`).
		WithArgs(id, 1).
		WillReturnError(errors.New("connection refused"))

	rec := patchCustomer(t, h, id, `{"email":"new@example.com"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := errorBody(t, rec); got != errInternal.Error() {
		t.Errorf("error = %q, want %q", got, errInternal.Error())
	}
	assertMockMet(t, mock)
}

func TestUpdate_SaveDatabaseFailure(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"

	h, mock := newTestHandlers(t)
	mock.ExpectQuery(`SELECT \* FROM "customers"`).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).
			AddRow(id, "Mary Smith", "old@example.com"))
	mock.ExpectExec(`UPDATE "customers" SET "email"=\$1 WHERE "customers"\."id" = \$2`).
		WithArgs("new@example.com", id).
		WillReturnError(errors.New("connection refused"))

	rec := patchCustomer(t, h, id, `{"email":"new@example.com"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := errorBody(t, rec); got != errInternal.Error() {
		t.Errorf("error = %q, want %q", got, errInternal.Error())
	}
	assertMockMet(t, mock)
}

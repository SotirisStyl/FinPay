package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	models "FinPay/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/labstack/echo"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const testCustomerID = "11111111-1111-4111-8111-111111111111"

func newLoanTestHandlers(t *testing.T) (handlers, sqlmock.Sqlmock) {
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

func getCustomerLoans(t *testing.T, h handlers, customerID string) *httptest.ResponseRecorder {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/customers/"+customerID+"/loans", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/customers/:id/loans")
	c.SetParamNames("id")
	c.SetParamValues(customerID)

	if err := h.getCustomerLoans(c); err != nil {
		t.Fatalf("getCustomerLoans returned error: %v", err)
	}
	return rec
}

func expectCustomerExists(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT \* FROM "customers" WHERE customers\.id = \$1 AND customers\.deleted_at IS NULL ORDER BY "customers"\."id" LIMIT \$2`).
		WithArgs(testCustomerID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "deleted_at"}).AddRow(testCustomerID, nil))
}

func expectLoanQuery(mock sqlmock.Sqlmock, rows *sqlmock.Rows) {
	mock.ExpectQuery(`SELECT \* FROM "loans" WHERE "loans"\."customer_id" = \$1`).
		WithArgs(testCustomerID).
		WillReturnRows(rows)
}

func assertLoanMockMet(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet DB expectations: %v", err)
	}
}

func TestGetCustomerLoans_Success(t *testing.T) {
	h, mock := newLoanTestHandlers(t)
	expectCustomerExists(mock)
	expectLoanQuery(mock, sqlmock.NewRows([]string{"id", "customer_id", "amount", "status"}).
		AddRow("22222222-2222-4222-8222-222222222222", testCustomerID, 125.50, "approved"))

	rec := getCustomerLoans(t, h, testCustomerID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var loans []models.Loan
	if err := json.Unmarshal(rec.Body.Bytes(), &loans); err != nil {
		t.Fatalf("response is not valid loan JSON: %v", err)
	}
	if len(loans) != 1 {
		t.Fatalf("loan count = %d, want 1", len(loans))
	}
	if loans[0].CustomerID != testCustomerID {
		t.Errorf("customer_id = %q, want %q", loans[0].CustomerID, testCustomerID)
	}
	assertLoanMockMet(t, mock)
}

func TestGetCustomerLoans_NoLoans(t *testing.T) {
	h, mock := newLoanTestHandlers(t)
	expectCustomerExists(mock)
	expectLoanQuery(mock, sqlmock.NewRows([]string{"id", "customer_id", "amount", "status"}))

	rec := getCustomerLoans(t, h, testCustomerID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "[]\n" {
		t.Errorf("body = %q, want an empty JSON array", rec.Body.String())
	}
	assertLoanMockMet(t, mock)
}

func TestGetCustomerLoans_CustomerNotFound(t *testing.T) {
	h, mock := newLoanTestHandlers(t)
	mock.ExpectQuery(`SELECT \* FROM "customers" WHERE customers\.id = \$1 AND customers\.deleted_at IS NULL ORDER BY "customers"\."id" LIMIT \$2`).
		WithArgs(testCustomerID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "deleted_at"}))

	rec := getCustomerLoans(t, h, testCustomerID)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	assertLoanMockMet(t, mock)
}

func TestGetCustomerLoans_InvalidUUID(t *testing.T) {
	h, mock := newLoanTestHandlers(t)

	rec := getCustomerLoans(t, h, "not-a-uuid")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertLoanMockMet(t, mock)
}

func TestGetCustomerLoans_CustomerQueryFailure(t *testing.T) {
	h, mock := newLoanTestHandlers(t)
	mock.ExpectQuery(`SELECT \* FROM "customers" WHERE customers\.id = \$1 AND customers\.deleted_at IS NULL ORDER BY "customers"\."id" LIMIT \$2`).
		WithArgs(testCustomerID, 1).
		WillReturnError(errors.New("connection refused"))

	rec := getCustomerLoans(t, h, testCustomerID)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	assertLoanMockMet(t, mock)
}

func TestGetCustomerLoans_LoanQueryFailure(t *testing.T) {
	h, mock := newLoanTestHandlers(t)
	expectCustomerExists(mock)
	mock.ExpectQuery(`SELECT \* FROM "loans" WHERE "loans"\."customer_id" = \$1`).
		WithArgs(testCustomerID).
		WillReturnError(errors.New("connection refused"))

	rec := getCustomerLoans(t, h, testCustomerID)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	assertLoanMockMet(t, mock)
}

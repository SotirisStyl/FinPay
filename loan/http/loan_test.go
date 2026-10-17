package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	models "FinPay/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/labstack/echo"
	"github.com/shopspring/decimal"
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

func postCustomerLoan(t *testing.T, h handlers, customerID, body string) *httptest.ResponseRecorder {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/customers/"+customerID+"/loans", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/customers/:id/loans")
	c.SetParamNames("id")
	c.SetParamValues(customerID)

	if err := h.createCustomerLoan(c); err != nil {
		t.Fatalf("createCustomerLoan returned error: %v", err)
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

func TestCreateCustomerLoan_Success(t *testing.T) {
	h, mock := newLoanTestHandlers(t)
	expectCustomerExists(mock)
	mock.ExpectQuery(`INSERT INTO "loans" \("customer_id","amount","status"\) VALUES \(\$1,\$2,\$3\) RETURNING "id"`).
		WithArgs(testCustomerID, sqlmock.AnyArg(), string(models.LoanStatusPending)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("22222222-2222-4222-8222-222222222222"))

	rec := postCustomerLoan(t, h, testCustomerID, `{"amount":125.5000}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var loan models.Loan
	if err := json.Unmarshal(rec.Body.Bytes(), &loan); err != nil {
		t.Fatalf("response is not valid loan JSON: %v", err)
	}
	if loan.ID != "22222222-2222-4222-8222-222222222222" {
		t.Errorf("id = %q, want database-generated ID", loan.ID)
	}
	if loan.CustomerID != testCustomerID {
		t.Errorf("customer_id = %q, want URL customer ID %q", loan.CustomerID, testCustomerID)
	}
	if loan.Status != string(models.LoanStatusPending) {
		t.Errorf("status = %q, want %q", loan.Status, models.LoanStatusPending)
	}
	wantAmount := decimal.NewFromFloat(125.50)
	if !loan.Amount.Equal(wantAmount) {
		t.Errorf("amount = %s, want %s", loan.Amount, wantAmount)
	}
	assertLoanMockMet(t, mock)
}

func TestCreateCustomerLoan_InvalidUUID(t *testing.T) {
	h, mock := newLoanTestHandlers(t)

	rec := postCustomerLoan(t, h, "not-a-uuid", `{"amount":10}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertLoanMockMet(t, mock)
}

func TestCreateCustomerLoan_CustomerNotFound(t *testing.T) {
	h, mock := newLoanTestHandlers(t)
	mock.ExpectQuery(`SELECT \* FROM "customers" WHERE customers\.id = \$1 AND customers\.deleted_at IS NULL ORDER BY "customers"\."id" LIMIT \$2`).
		WithArgs(testCustomerID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "deleted_at"}))

	rec := postCustomerLoan(t, h, testCustomerID, `{"amount":10}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	assertLoanMockMet(t, mock)
}

func TestCreateCustomerLoan_MalformedJSON(t *testing.T) {
	h, mock := newLoanTestHandlers(t)

	rec := postCustomerLoan(t, h, testCustomerID, `{"amount":`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertLoanMockMet(t, mock)
}

func TestCreateCustomerLoan_MissingAmount(t *testing.T) {
	h, mock := newLoanTestHandlers(t)

	rec := postCustomerLoan(t, h, testCustomerID, `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertLoanMockMet(t, mock)
}

func TestCreateCustomerLoan_NonPositiveAmount(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "zero", body: `{"amount":0}`},
		{name: "negative", body: `{"amount":-1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, mock := newLoanTestHandlers(t)

			rec := postCustomerLoan(t, h, testCustomerID, tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			assertLoanMockMet(t, mock)
		})
	}
}

func TestCreateCustomerLoan_AmountHasMoreThanTwoFractionalDigits(t *testing.T) {
	h, mock := newLoanTestHandlers(t)

	rec := postCustomerLoan(t, h, testCustomerID, `{"amount":1.234}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertLoanMockMet(t, mock)
}

func TestCreateCustomerLoan_AmountExceedsDatabasePrecision(t *testing.T) {
	h, mock := newLoanTestHandlers(t)

	rec := postCustomerLoan(t, h, testCustomerID, `{"amount":100000000}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertLoanMockMet(t, mock)
}

func TestCreateCustomerLoan_StatusNotAccepted(t *testing.T) {
	h, mock := newLoanTestHandlers(t)

	rec := postCustomerLoan(t, h, testCustomerID, `{"amount":10,"status":"APPROVED"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertLoanMockMet(t, mock)
}

func TestCreateCustomerLoan_UnknownFieldNotAccepted(t *testing.T) {
	h, mock := newLoanTestHandlers(t)

	rec := postCustomerLoan(t, h, testCustomerID, `{"amount":10,"customer_id":"22222222-2222-4222-8222-222222222222"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertLoanMockMet(t, mock)
}

func TestCreateCustomerLoan_AdditionalJSONValueNotAccepted(t *testing.T) {
	h, mock := newLoanTestHandlers(t)

	rec := postCustomerLoan(t, h, testCustomerID, `{"amount":10} {"amount":20}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertLoanMockMet(t, mock)
}

func TestCreateCustomerLoan_CustomerQueryFailure(t *testing.T) {
	h, mock := newLoanTestHandlers(t)
	mock.ExpectQuery(`SELECT \* FROM "customers" WHERE customers\.id = \$1 AND customers\.deleted_at IS NULL ORDER BY "customers"\."id" LIMIT \$2`).
		WithArgs(testCustomerID, 1).
		WillReturnError(errors.New("connection refused"))

	rec := postCustomerLoan(t, h, testCustomerID, `{"amount":10}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	assertLoanMockMet(t, mock)
}

func TestCreateCustomerLoan_LoanCreationFailure(t *testing.T) {
	h, mock := newLoanTestHandlers(t)
	expectCustomerExists(mock)
	mock.ExpectQuery(`INSERT INTO "loans" \("customer_id","amount","status"\) VALUES \(\$1,\$2,\$3\) RETURNING "id"`).
		WithArgs(testCustomerID, sqlmock.AnyArg(), string(models.LoanStatusPending)).
		WillReturnError(errors.New("connection refused"))

	rec := postCustomerLoan(t, h, testCustomerID, `{"amount":10}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	assertLoanMockMet(t, mock)
}

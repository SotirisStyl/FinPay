package http

import (
	"FinPay/config"
	"FinPay/loan/scopes"
	"FinPay/services"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	models "FinPay/models"

	"github.com/google/uuid"
	"github.com/labstack/echo"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var (
	errInvalidUUID          = errors.New("invalid uuid")
	errLoanCustomerNotFound = errors.New("customer does not exist")
	errInternal             = errors.New("internal server error")
	errAmountRequired       = errors.New("amount is required")
	errInvalidAmount        = errors.New("amount must be greater than 0")
	errAmountPrecision      = errors.New("amount must not have more than 2 decimal places")
	errAmountTooLarge       = errors.New("amount exceeds the maximum allowed value")
	errStatusInJSON         = errors.New("status should not be given for loan")
	errLoanExists           = errors.New("loan already exists")
	errInvalidOutput        = errors.New("invalid output")
	maxLoanAmount           = decimal.RequireFromString("99999999.99")
	errLoanDoesNotExist     = errors.New("loan does not exist")
	errStatusRequired       = errors.New("status is required")
	errInvalidLoanStatus    = errors.New("invalid loan status")
	errInvalidTransition    = errors.New("invalid loan status transition")
	errAlreadyUpdated       = errors.New("Loan status has changed. Please refresh and try again.")
)

type handlers struct {
	DB         *gorm.DB
	coreConfig config.CoreConfig
}

func NewHandlers(db *gorm.DB, coreConfig config.CoreConfig) services.Handlers {
	return &handlers{DB: db, coreConfig: coreConfig}
}

func mapCreateError(err error) error {
	if err == nil {
		return nil
	}

	if isUniqueViolation(err) {
		return errLoanExists
	}

	return errInternal
}

func isUniqueViolation(err error) bool {
	var sqlStateErr interface {
		SQLState() string
	}
	return errors.As(err, &sqlStateErr) && sqlStateErr.SQLState() == "23505"
}

func canTransitionLoanStatus(from, to models.LoanStatus) bool {
	switch from {
	case models.LoanStatusPending:
		return to == models.LoanStatusApproved || to == models.LoanStatusRejected
	case models.LoanStatusApproved:
		return to == models.LoanStatusActive
	case models.LoanStatusActive:
		return to == models.LoanStatusPaidOff
	default:
		return false
	}
}

func isValidLoanStatus(status models.LoanStatus) bool {
	switch status {
	case models.LoanStatusPending, models.LoanStatusApproved, models.LoanStatusRejected,
		models.LoanStatusActive, models.LoanStatusPaidOff:
		return true
	default:
		return false
	}
}

func (h handlers) getCustomerLoans(c echo.Context) error {
	customerID := c.Param("id")
	if err := uuid.Validate(customerID); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errInvalidUUID.Error()})
	}

	var customer models.Customer
	result := h.DB.Scopes(scopes.ByCustomerID(customerID)).First(&customer)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": errLoanCustomerNotFound.Error()})
	}
	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errInternal.Error()})
	}

	loans := make([]models.Loan, 0)
	if err := h.DB.Model(&customer).Association("Loans").Find(&loans); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errInternal.Error()})
	}

	if len(loans) == 0 {
		return c.JSON(http.StatusOK, []models.Loan{})
	}

	return c.JSON(http.StatusOK, loans)
}

func (h handlers) createCustomerLoan(c echo.Context) error {
	customerID := c.Param("id")
	var customer models.Customer

	if err := uuid.Validate(customerID); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errInvalidUUID.Error()})
	}

	decoder := json.NewDecoder(c.Request().Body)
	decoder.DisallowUnknownFields()

	var request models.CreateLoanRequest
	if err := decoder.Decode(&request); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errInvalidOutput.Error()})
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errInvalidOutput.Error()})
	}

	if request.Amount == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errAmountRequired.Error()})
	}
	if !request.Amount.IsPositive() {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errInvalidAmount.Error()})
	}
	if !request.Amount.Equal(request.Amount.Round(2)) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errAmountPrecision.Error()})
	}
	if request.Amount.GreaterThan(maxLoanAmount) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errAmountTooLarge.Error()})
	}
	if request.Status != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errStatusInJSON.Error()})
	}

	validateCustomer := h.DB.Scopes(scopes.ByCustomerID(customerID)).First(&customer)
	if errors.Is(validateCustomer.Error, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": errLoanCustomerNotFound.Error()})
	}
	if validateCustomer.Error != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errInternal.Error()})
	}

	loan := models.Loan{
		CustomerID: customerID,
		Amount:     *request.Amount,
		Status:     string(models.LoanStatusPending),
	}

	if err := h.DB.Create(&loan).Error; err != nil {
		mapped := mapCreateError(err)

		status := http.StatusInternalServerError
		if errors.Is(mapped, errLoanExists) {
			status = http.StatusConflict
		}

		return c.JSON(status, map[string]string{
			"error": mapped.Error(),
		})
	}

	return c.JSON(http.StatusCreated, loan)
}

func (h handlers) updateLoan(c echo.Context) error {
	loanID := c.Param("id")
	var loan models.Loan
	if err := uuid.Validate(loanID); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errInvalidUUID.Error()})
	}

	decoder := json.NewDecoder(c.Request().Body)
	decoder.DisallowUnknownFields()

	var request models.UpdateLoan
	if err := decoder.Decode(&request); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errInvalidOutput.Error()})
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errInvalidOutput.Error()})
	}

	if request.Status == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errStatusRequired.Error()})
	}
	if !isValidLoanStatus(*request.Status) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errInvalidLoanStatus.Error()})
	}

	result := h.DB.Scopes(scopes.ByLoanID(loanID)).First(&loan)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": errLoanDoesNotExist.Error()})
	}
	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errInternal.Error()})
	}

	currentStatus := models.LoanStatus(loan.Status)
	if !canTransitionLoanStatus(currentStatus, *request.Status) {
		return c.JSON(http.StatusConflict, map[string]string{"error": errInvalidTransition.Error()})
	}

	update := h.DB.Model(&models.Loan{}).
		Where(&models.Loan{ID: loanID, Status: string(currentStatus)}).
		UpdateColumn("Status", *request.Status)
	if update.Error != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errInternal.Error()})
	}
	if update.RowsAffected == 0 {
		return c.JSON(http.StatusConflict, map[string]string{"error": errAlreadyUpdated.Error()})
	}

	loan.Status = string(*request.Status)
	return c.JSON(http.StatusOK, loan)
}

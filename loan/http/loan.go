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
	"gorm.io/gorm"
)

var (
	errInvalidUUID          = errors.New("invalid uuid")
	errLoanCustomerNotFound = errors.New("customer does not exist")
	errInternal             = errors.New("internal server error")
	errAmountRequired       = errors.New("amount is required")
	errInvalidAmount        = errors.New("amount must be greater than 0")
	errStatusInJSON         = errors.New("status should not be given for loan")
	errLoanExists           = errors.New("loan already exists")
	errInvalidOutput        = errors.New("invalid output")
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

package http

import (
	"FinPay/config"
	"FinPay/services"
	"errors"
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
)

type handlers struct {
	DB         *gorm.DB
	coreConfig config.CoreConfig
}

func NewHandlers(db *gorm.DB, coreConfig config.CoreConfig) services.Handlers {
	return &handlers{DB: db, coreConfig: coreConfig}
}

func (h handlers) getCustomerLoans(c echo.Context) error {
	customerID := c.Param("id")
	if err := uuid.Validate(customerID); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errInvalidUUID.Error()})
	}

	var customer models.Customer
	result := h.DB.Where(&models.Customer{ID: customerID}).First(&customer)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": errLoanCustomerNotFound.Error()})
	}
	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errInternal.Error()})
	}

	loans := make([]models.Loan, 0)
	if err := h.DB.InnerJoins("Customer").
		Where(&models.Loan{CustomerID: customerID}).
		Find(&loans).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errInternal.Error()})
	}

	if len(loans) == 0 {
		return c.JSON(http.StatusOK, []models.Loan{})
	}

	return c.JSON(http.StatusOK, loans)
}

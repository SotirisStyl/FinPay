package http

import (
	"errors"
	"net/http"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/labstack/echo"
	"gorm.io/gorm"

	"FinPay/config"
	"FinPay/customer/scopes"
	"FinPay/services"
)

var (
	errNameTwoWords          = errors.New("customer name must be two words")
	errNameEmailRequired     = errors.New("name and email are required")
	errEmailExists           = errors.New("email already exists")
	errInternal              = errors.New("internal server error")
	errCustomerNotFound      = errors.New("customer does not exist")
	errInvalidUuid           = errors.New("invalid uuid")
	errCustomerAlreadyExists = errors.New("customer already exist")
)

type Customer struct {
	ID    string `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name  string `gorm:"not null" json:"name"`
	Email string `gorm:"uniqueIndex;not null" json:"email"`
}

type customerUpdate struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
}

type handlers struct {
	DB         *gorm.DB
	coreConfig config.CoreConfig
}

func NewHandlers(db *gorm.DB, coreConfig config.CoreConfig) services.Handlers {
	return &handlers{DB: db, coreConfig: coreConfig}
}

func normalizeName(name string) string {
	parts := strings.Fields(name)
	for i, part := range parts {
		if part == "" {
			continue
		}

		runes := []rune(strings.ToLower(part))
		if len(runes) == 0 {
			continue
		}

		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}

	return strings.Join(parts, " ")
}

func validateRequired(name, email string) error {
	if name == "" || email == "" {
		return errNameEmailRequired
	}
	return nil
}

func mapCreateError(err error) error {
	if err == nil {
		return nil
	}

	if isUniqueViolation(err) {
		return errEmailExists
	}

	return errInternal
}

func mapUpdateError(err error) error {
	if err == nil {
		return nil
	}

	if isUniqueViolation(err) {
		return errEmailExists
	}

	return errInternal
}

func isUniqueViolation(err error) bool {
	var sqlStateErr interface {
		SQLState() string
	}
	return errors.As(err, &sqlStateErr) && sqlStateErr.SQLState() == "23505"
}

func (h handlers) create(c echo.Context) error {
	var customer Customer

	if err := c.Bind(&customer); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid input",
		})
	}

	customer.Name = normalizeName(customer.Name)

	if err := validateRequired(customer.Name, customer.Email); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	if err := h.DB.Create(&customer).Error; err != nil {
		mapped := mapCreateError(err)

		status := http.StatusInternalServerError
		if errors.Is(mapped, errEmailExists) {
			status = http.StatusConflict
		}

		return c.JSON(status, map[string]string{
			"error": mapped.Error(),
		})
	}

	return c.JSON(http.StatusCreated, customer)
}

func (h handlers) get(c echo.Context) error {
	var customer Customer
	id := c.Param("id")

	if err := uuid.Validate(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errInvalidUuid.Error()})
	}

	result := h.DB.Scopes(scopes.ByID(id)).First(&customer)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": errCustomerNotFound.Error()})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errInternal.Error()})
	}

	return c.JSON(http.StatusOK, customer)
}

func (h handlers) update(c echo.Context) error {
	var updates customerUpdate
	id := c.Param("id")

	if err := uuid.Validate(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errInvalidUuid.Error()})
	}

	if err := c.Bind(&updates); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid input"})
	}

	var customer Customer
	result := h.DB.Scopes(scopes.ByID(id)).First(&customer)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": errCustomerNotFound.Error()})
	}
	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": errInternal.Error()})
	}

	if updates.Name != nil {
		customer.Name = normalizeName(*updates.Name)
	}
	if updates.Email != nil {
		customer.Email = *updates.Email
	}
	if err := validateRequired(customer.Name, customer.Email); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := h.DB.Save(&customer).Error; err != nil {
		mapped := mapUpdateError(err)

		status := http.StatusInternalServerError
		if errors.Is(mapped, errEmailExists) {
			status = http.StatusConflict
		}

		return c.JSON(status, map[string]string{
			"error": mapped.Error(),
		})
	}

	return c.JSON(http.StatusOK, customer)
}

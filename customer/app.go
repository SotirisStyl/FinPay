package http

import (
	"github.com/labstack/echo"
)

const (
	createCustomer = "/customers"
)

func (handlers) GetWhitelistPaths(_ string) (out []string) {
	return
}

func (h handlers) CreateAuthenticatedEndpoints(g *echo.Group) {
	g.POST(createCustomer, h.create)
}

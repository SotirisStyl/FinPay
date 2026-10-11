package http

import (
	"github.com/labstack/echo"
)

const (
	createCustomer = "/customers"
	getCustomer    = "/customers/:id"
	updateCustomer = "/customers/:id"
	deleteCustomer = "/customers/:id"
)

func (handlers) GetWhitelistPaths(_ string) (out []string) {
	return
}

func (h handlers) CreateAuthenticatedEndpoints(g *echo.Group) {
	g.POST(createCustomer, h.create)
	g.GET(getCustomer, h.get)
	g.PATCH(updateCustomer, h.update)
	g.DELETE(deleteCustomer, h.delete)
}

package http

import "github.com/labstack/echo"

const (
	createLoan = "/customers/:id/loans"
)

func (handlers) GetWhitelistPaths(_ string) (out []string) {
	return
}

func (h handlers) CreateAuthenticatedEndpoints(g *echo.Group) {
	g.GET(createLoan, h.getCustomerLoans)
}

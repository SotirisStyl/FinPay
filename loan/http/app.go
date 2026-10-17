package http

import "github.com/labstack/echo"

const (
	getLoans  = "/customers/:id/loans"
	postLoans = "/customers/:id/loans"
)

func (handlers) GetWhitelistPaths(_ string) (out []string) {
	return
}

func (h handlers) CreateAuthenticatedEndpoints(g *echo.Group) {
	g.GET(getLoans, h.getCustomerLoans)
	g.POST(postLoans, h.createCustomerLoan)
}

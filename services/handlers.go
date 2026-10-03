package services

import "github.com/labstack/echo"

type Handlers interface {
	GetWhitelistPaths(string) []string
	CreateAuthenticatedEndpoints(*echo.Group)
}

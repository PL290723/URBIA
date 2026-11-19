package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/middleware"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

func parseID(c *gin.Context, nombre string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(nombre), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "El identificador debe ser un entero positivo")
		return 0, false
	}
	return id, true
}

func sesionRequerida(c *gin.Context) (auth.Sesion, bool) {
	s, ok := middleware.SesionActual(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "Se requiere un token de acceso")
	}
	return s, ok
}

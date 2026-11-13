package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/middleware"
	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/response"
	"github.com/PL290723/URBIA/backend-go/internal/services"
)

type AuthHandler struct{ svc *services.AuthService }

func NewAuthHandler(svc *services.AuthService) *AuthHandler { return &AuthHandler{svc: svc} }

func (h *AuthHandler) Registro(c *gin.Context) {
	var req models.RegistroRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.svc.Registrar(c.Request.Context(), req)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.Created(c, res)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.svc.IniciarSesion(c.Request.Context(), req)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, res)
}

func (h *AuthHandler) Perfil(c *gin.Context) {
	sesion, ok := middleware.SesionActual(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "Se requiere un token de acceso")
		return
	}
	u, err := h.svc.Perfil(c.Request.Context(), sesion.UsuarioID)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, u)
}

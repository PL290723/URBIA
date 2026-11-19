package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/response"
	"github.com/PL290723/URBIA/backend-go/internal/services"
)

type UsuarioHandler struct{ svc *services.UsuarioService }

func NewUsuarioHandler(svc *services.UsuarioService) *UsuarioHandler {
	return &UsuarioHandler{svc: svc}
}

func (h *UsuarioHandler) Listar(c *gin.Context) {
	usuarios, err := h.svc.Listar(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, usuarios)
}

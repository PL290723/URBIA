package router

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/handlers"
	"github.com/PL290723/URBIA/backend-go/internal/middleware"
)

// Todo lo que cuelga de /admin exige token y rol administrador.
func registerAdminRoutes(rg *gin.RouterGroup, h *handlers.UsuarioHandler, requireAuth gin.HandlerFunc) {
	g := rg.Group("/admin", requireAuth, middleware.RequireRol(dominio.RolAdministrador))
	g.GET("/usuarios", h.Listar)
}

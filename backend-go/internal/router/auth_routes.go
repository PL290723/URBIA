package router

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/handlers"
)

func registerAuthRoutes(rg *gin.RouterGroup, h *handlers.AuthHandler, requireAuth gin.HandlerFunc) {
	g := rg.Group("/auth")
	g.POST("/registro", h.Registro)
	g.POST("/login", h.Login)
	g.GET("/perfil", requireAuth, h.Perfil)
}

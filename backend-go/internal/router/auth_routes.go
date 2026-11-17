package router

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/handlers"
)

// El login lleva límite de intentos fallidos por IP contra la fuerza bruta (HU 09.1).
func registerAuthRoutes(rg *gin.RouterGroup, h *handlers.AuthHandler, requireAuth, limiteLogin gin.HandlerFunc) {
	g := rg.Group("/auth")
	g.POST("/registro", h.Registro)
	g.POST("/login", limiteLogin, h.Login)
	g.GET("/perfil", requireAuth, h.Perfil)
}

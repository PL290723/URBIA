package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

const sesionKey = "sesion"

// RequireAuth exige un JWT válido en "Authorization: Bearer <token>".
func RequireAuth(tokens *auth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		tokenStr = strings.TrimSpace(tokenStr)
		if !ok || tokenStr == "" {
			noAutorizado(c, "Se requiere un token de acceso")
			return
		}
		sesion, err := tokens.Validar(tokenStr)
		if err != nil {
			noAutorizado(c, "Token inválido o expirado")
			return
		}
		c.Set(sesionKey, sesion)
		c.Next()
	}
}

// RequireRol deja pasar solo a los roles indicados. Va después de RequireAuth.
func RequireRol(roles ...dominio.Rol) gin.HandlerFunc {
	return RequirePermiso(func(r dominio.Rol) bool { return slices.Contains(roles, r) })
}

// RequirePermiso aplica una regla de dominio, p. ej. dominio.Rol.PuedeCambiarEstado.
func RequirePermiso(permitido func(dominio.Rol) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		sesion, ok := SesionActual(c)
		if !ok {
			noAutorizado(c, "Se requiere un token de acceso")
			return
		}
		if !permitido(sesion.Rol) {
			response.Error(c, http.StatusForbidden, response.CodeForbidden, "No tienes permisos para realizar esta acción")
			return
		}
		c.Next()
	}
}

func SesionActual(c *gin.Context) (auth.Sesion, bool) {
	v, ok := c.Get(sesionKey)
	if !ok {
		return auth.Sesion{}, false
	}
	s, ok := v.(auth.Sesion)
	return s, ok
}

func noAutorizado(c *gin.Context, msg string) {
	c.Header("WWW-Authenticate", `Bearer realm="urbia"`)
	response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, msg)
}

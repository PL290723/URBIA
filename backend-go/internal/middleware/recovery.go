package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/response"
)

func Recovery(log *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, rec any) {
		log.Error("panic recuperado",
			"error", rec,
			"path", c.Request.URL.Path,
			"request_id", c.GetString(RequestIDKey),
			"stack", string(debug.Stack()),
		)
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Ocurrió un error inesperado")
	})
}

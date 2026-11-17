package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"

	"github.com/gin-gonic/gin"
)

const (
	RequestIDKey    = "request_id"
	RequestIDHeader = "X-Request-ID"
)

// patronRequestID: solo se reutiliza el id del cliente si es corto y "limpio".
// Así nadie mete saltos de línea ni texto arbitrario en los logs (HU 09.1).
var patronRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if !patronRequestID.MatchString(id) {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		c.Set(RequestIDKey, id)
		c.Header(RequestIDHeader, id)
		c.Next()
	}
}

package middleware

import (
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

const (
	// LimiteCuerpoJSON: ningún JSON de la API se le acerca (el texto más largo,
	// la descripción de un reporte, es de 1000 caracteres).
	LimiteCuerpoJSON int64 = 64 << 10 // 64 KiB

	// LimiteCuerpoMultipart: la foto (10 MiB) más 1 MiB para los demás campos
	// y los encabezados del multipart. Es el mismo margen que usan los handlers.
	LimiteCuerpoMultipart = dominio.TamanoMaximoArchivo + 1<<20

	msgCuerpoGrande = "El cuerpo de la solicitud es demasiado grande"
)

// LimiteCuerpo pone un tope de bytes a TODA petición (HU 09.1), para que nadie
// llene la memoria del servidor mandando un JSON gigante.
//
// Los multipart (fotos) reciben el tope de imagen; el 413 con el mensaje de
// "imagen de más de 10 MB" lo siguen dando sus handlers. El resto recibe 64 KiB.
// Mandar un JSON con Content-Type multipart no sirve para saltarse el límite:
// como máximo leería LimiteCuerpoMultipart.
func LimiteCuerpo() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body == nil || c.Request.Body == http.NoBody {
			c.Next()
			return
		}

		limite := LimiteCuerpoJSON
		if esMultipart(c.Request) {
			limite = LimiteCuerpoMultipart
		} else if c.Request.ContentLength > limite {
			response.Error(c, http.StatusRequestEntityTooLarge, response.CodeBodyTooLarge, msgCuerpoGrande)
			return
		}

		// Si el cliente no declara Content-Length (chunked), el corte llega al leer:
		// handlers.bindJSON traduce el *http.MaxBytesError a 413.
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limite)
		c.Next()
	}
}

func esMultipart(r *http.Request) bool {
	tipo, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && tipo == "multipart/form-data"
}

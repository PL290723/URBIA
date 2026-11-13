package handlers

import (
	"errors"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"github.com/PL290723/URBIA/backend-go/internal/response"
)

func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(func(f reflect.StructField) string {
			nombre := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
			if nombre == "-" {
				return ""
			}
			return nombre
		})
	}
}

func bindJSON(c *gin.Context, dst any) bool {
	err := c.ShouldBindJSON(dst)
	if err == nil {
		return true
	}
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		detalles := make([]gin.H, 0, len(ve))
		for _, fe := range ve {
			detalles = append(detalles, gin.H{"campo": fe.Field(), "regla": fe.Tag()})
		}
		response.ValidationError(c, detalles)
		return false
	}
	_ = c.Error(err) // el logger muestra el motivo real (p. ej. "invalid character 'n'...")
	response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "El cuerpo debe ser un JSON válido")
	return false
}

package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AppError es el error que devuelven los services; el handler lo traduce a HTTP.
type AppError struct {
	Status  int
	Code    string
	Message string
}

func (e *AppError) Error() string { return e.Message }

func BadRequest(msg string) *AppError {
	return &AppError{Status: http.StatusBadRequest, Code: CodeBadRequest, Message: msg}
}
func Unauthorized(msg string) *AppError {
	return &AppError{Status: http.StatusUnauthorized, Code: CodeUnauthorized, Message: msg}
}
func Forbidden(msg string) *AppError {
	return &AppError{Status: http.StatusForbidden, Code: CodeForbidden, Message: msg}
}
func NotFound(msg string) *AppError {
	return &AppError{Status: http.StatusNotFound, Code: CodeNotFound, Message: msg}
}
func Conflict(msg string) *AppError {
	return &AppError{Status: http.StatusConflict, Code: CodeConflict, Message: msg}
}

// HandleError responde con el AppError si lo es; si no, 500 sin filtrar detalles internos.
func HandleError(c *gin.Context, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		Error(c, appErr.Status, appErr.Code, appErr.Message)
		return
	}
	_ = c.Error(err) // el middleware de logging lo registra
	Error(c, http.StatusInternalServerError, CodeInternal, "Ocurrió un error inesperado")
}

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
	Details any // opcional: errores por campo
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

// Validation es el mismo 400 VALIDACION_FALLIDA de ValidationError, pero devuelto desde un service.
func Validation(details any) *AppError {
	return &AppError{Status: http.StatusBadRequest, Code: CodeValidation,
		Message: "Los datos enviados no son válidos", Details: details}
}
func PayloadTooLarge(msg string) *AppError {
	return &AppError{Status: http.StatusRequestEntityTooLarge, Code: CodePayloadTooLarge, Message: msg}
}

// HandleError responde con el AppError si lo es; si no, 500 sin filtrar detalles internos.
func HandleError(c *gin.Context, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		c.AbortWithStatusJSON(appErr.Status, Body{Error: &ErrorBody{
			Code: appErr.Code, Message: appErr.Message, Details: appErr.Details,
		}})
		return
	}
	_ = c.Error(err) // el middleware de logging lo registra
	Error(c, http.StatusInternalServerError, CodeInternal, "Ocurrió un error inesperado")
}

func UnsupportedMediaType(msg string) *AppError {
	return &AppError{Status: http.StatusUnsupportedMediaType, Code: CodeUnsupportedMedia, Message: msg}
}
func UnprocessableImage(msg string) *AppError {
	return &AppError{Status: http.StatusUnprocessableEntity, Code: CodeInvalidImage, Message: msg}
}

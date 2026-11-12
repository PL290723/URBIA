package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Body es el formato único de todas las respuestas de la API.
type Body struct {
	Success bool       `json:"success"`
	Data    any        `json:"data,omitempty"`
	Error   *ErrorBody `json:"error,omitempty"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

const (
	CodeBadRequest         = "SOLICITUD_INVALIDA"
	CodeValidation         = "VALIDACION_FALLIDA"
	CodeUnauthorized       = "NO_AUTORIZADO"
	CodeForbidden          = "PROHIBIDO"
	CodeNotFound           = "NO_ENCONTRADO"
	CodeMethodNotAllowed   = "METODO_NO_PERMITIDO"
	CodeConflict           = "CONFLICTO"
	CodeInternal           = "ERROR_INTERNO"
	CodeServiceUnavailable = "SERVICIO_NO_DISPONIBLE"
)

func OK(c *gin.Context, data any)      { c.JSON(http.StatusOK, Body{Success: true, Data: data}) }
func Created(c *gin.Context, data any) { c.JSON(http.StatusCreated, Body{Success: true, Data: data}) }
func NoContent(c *gin.Context)         { c.Status(http.StatusNoContent) }

func Error(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, Body{Error: &ErrorBody{Code: code, Message: message}})
}

func ValidationError(c *gin.Context, details any) {
	c.AbortWithStatusJSON(http.StatusBadRequest, Body{Error: &ErrorBody{
		Code: CodeValidation, Message: "Los datos enviados no son válidos", Details: details,
	}})
}

package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Se cargan variables de entorno
	if err := godotenv.Load(); err != nil {
		log.Println("Aviso: No se encontró el archivo .env, usando variables del sistema.")
	}

	gin.SetMode(os.Getenv("GIN_MODE"))

	// Inicializar router con middleware por defecto
	// Logger(): Formato y logging estructurado de peticiones HTTP
	// Recovery(): Manejo de errores para evitar que el servidor colapse ante un panic
	router := gin.Default()

	// Grupo de rutas para la API
	api := router.Group("/api/v1")
	{
		// Endpoint inicial GET /api/health (Subtarea 5)
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "ok",
				"message": "URBIA API funcionando correctamente",
			})
		})
	}

	// Configuración de puerto
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Servidor iniciando en el puerto %s...", port)

	// Arrancar el servidor
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Error al arrancar el servidor: %v", err)
	}
}
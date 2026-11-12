package models

import "time"

type RegistroRequest struct {
	Nombre     string `json:"nombre" binding:"required,max=100"`
	Correo     string `json:"correo" binding:"required,email,max=150"`
	Contrasena string `json:"contrasena" binding:"required,min=8,max=72"`
}

type LoginRequest struct {
	Correo     string `json:"correo" binding:"required,email,max=150"`
	Contrasena string `json:"contrasena" binding:"required,max=72"`
}

type AuthResponse struct {
	Token     string    `json:"token"`
	TipoToken string    `json:"tipo_token"`
	ExpiraEn  time.Time `json:"expira_en"`
	Usuario   Usuario   `json:"usuario"`
}

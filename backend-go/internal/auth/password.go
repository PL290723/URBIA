package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

var ErrContrasenaIncorrecta = errors.New("contraseña incorrecta")

// HashContrasena genera el hash bcrypt; la contraseña en texto plano nunca se guarda.
func HashContrasena(plana string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plana), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func CompararContrasena(hash, plana string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plana)); err != nil {
		return ErrContrasenaIncorrecta
	}
	return nil
}

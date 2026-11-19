package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
)

var (
	ErrNoEncontrado    = errors.New("registro no encontrado")
	ErrCorreoDuplicado = errors.New("correo ya registrado")
)

const columnasUsuario = `id_usuario, nombre, correo, hash_contrasena, rol, fecha_registro, fecha_actualizacion`

type UsuarioRepository struct{ db *pgxpool.Pool }

func NewUsuarioRepository(db *pgxpool.Pool) *UsuarioRepository { return &UsuarioRepository{db: db} }

func (r *UsuarioRepository) Crear(ctx context.Context, u *models.Usuario) error {
	const q = `INSERT INTO usuario (nombre, correo, hash_contrasena, rol)
	           VALUES ($1, $2, $3, $4)
	           RETURNING id_usuario, fecha_registro, fecha_actualizacion`
	err := r.db.QueryRow(ctx, q, u.Nombre, u.Correo, u.HashContrasena, string(u.Rol)).
		Scan(&u.ID, &u.FechaRegistro, &u.FechaActualizacion)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation (uq_usuario_correo)
		return ErrCorreoDuplicado
	}
	if err != nil {
		return fmt.Errorf("crear usuario: %w", err)
	}
	return nil
}

func (r *UsuarioRepository) BuscarPorCorreo(ctx context.Context, correo string) (*models.Usuario, error) {
	q := `SELECT ` + columnasUsuario + ` FROM usuario WHERE lower(correo) = lower($1)`
	return scanUsuario(r.db.QueryRow(ctx, q, correo))
}

func (r *UsuarioRepository) BuscarPorID(ctx context.Context, id int64) (*models.Usuario, error) {
	q := `SELECT ` + columnasUsuario + ` FROM usuario WHERE id_usuario = $1`
	return scanUsuario(r.db.QueryRow(ctx, q, id))
}

func (r *UsuarioRepository) Listar(ctx context.Context) ([]models.Usuario, error) {
	rows, err := r.db.Query(ctx, `SELECT `+columnasUsuario+` FROM usuario ORDER BY id_usuario`)
	if err != nil {
		return nil, fmt.Errorf("listar usuarios: %w", err)
	}
	defer rows.Close()

	usuarios := []models.Usuario{}
	for rows.Next() {
		u, err := scanUsuario(rows)
		if err != nil {
			return nil, err
		}
		usuarios = append(usuarios, *u)
	}
	return usuarios, rows.Err()
}

func scanUsuario(row pgx.Row) (*models.Usuario, error) {
	var u models.Usuario
	var rol string
	err := row.Scan(&u.ID, &u.Nombre, &u.Correo, &u.HashContrasena, &rol, &u.FechaRegistro, &u.FechaActualizacion)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("leer usuario: %w", err)
	}
	u.Rol = dominio.Rol(rol)
	return &u, nil
}

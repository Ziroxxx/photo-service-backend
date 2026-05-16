package postgres

import (
	"context"
	"strings"
	"time"

	"photo-service-back/domain/user"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, u *user.User) (*user.User, error) {
	q := `
		INSERT INTO users (login, password_hash, full_name, role, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, login, password_hash, full_name, role, status, created_at, updated_at, last_login_at
	`
	row := r.db.QueryRow(ctx, q, u.Login, u.PasswordHash, u.FullName, u.Role, u.Status)

	var out user.User
	err := row.Scan(
		&out.ID, &out.Login, &out.PasswordHash, &out.FullName,
		&out.Role, &out.Status, &out.CreatedAt, &out.UpdatedAt, &out.LastLoginAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "users_login_key") {
			return nil, user.ErrLoginAlreadyExists
		}
		return nil, err
	}
	return &out, nil
}

func (r *UserRepo) GetByLogin(ctx context.Context, login string) (*user.User, error) {
	q := `
		SELECT id, login, password_hash, full_name, phone, role, status, created_at, updated_at, last_login_at
		FROM users
		WHERE login = $1
	`

	var out user.User
	err := r.db.QueryRow(ctx, q, login).Scan(
		&out.ID, &out.Login, &out.PasswordHash, &out.FullName, &out.Phone,
		&out.Role, &out.Status, &out.CreatedAt, &out.UpdatedAt, &out.LastLoginAt,
	)
	if err != nil {
		return nil, user.ErrUserNotFound
	}
	return &out, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	q := `
		SELECT id, email, password_hash, full_name, phone, role, status, created_at, updated_at, last_login_at
		FROM users
		WHERE email = $1
	`
	var out user.User
	err := r.db.QueryRow(ctx, q, email).Scan(
		&out.ID, &out.Email, &out.PasswordHash, &out.FullName, &out.Phone,
		&out.Role, &out.Status, &out.CreatedAt, &out.UpdatedAt, &out.LastLoginAt,
	)
	if err != nil {
		return nil, user.ErrUserNotFound
	}
	return &out, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	q := `
		SELECT id, login, password_hash, full_name, phone, role, status, created_at, updated_at, last_login_at
		FROM users
		WHERE id = $1
	`

	var out user.User
	err := r.db.QueryRow(ctx, q, id).Scan(
		&out.ID, &out.Login, &out.PasswordHash, &out.FullName, &out.Phone,
		&out.Role, &out.Status, &out.CreatedAt, &out.UpdatedAt, &out.LastLoginAt,
	)
	if err != nil {
		return nil, user.ErrUserNotFound
	}
	return &out, nil
}

func (r *UserRepo) List(ctx context.Context, filter user.ListUsersFilter) ([]user.User, error) {
	q := `
		SELECT id, login, email, password_hash, full_name, phone, role, status, created_at, updated_at, last_login_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, q, filter.PageSize, (filter.Page-1)*filter.PageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []user.User
	for rows.Next() {
		var u user.User
		if err := rows.Scan(
			&u.ID, &u.Login, &u.Email, &u.PasswordHash, &u.FullName, &u.Phone,
			&u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt,
		); err != nil {
			return nil, err
		}
		items = append(items, u)
	}
	return items, rows.Err()
}

func (r *UserRepo) UpdateRole(ctx context.Context, id uuid.UUID, role user.Role) (*user.User, error) {
	q := `
		UPDATE users
		SET role = $2
		WHERE id = $1
		RETURNING id, login, email, password_hash, full_name, phone, role, status, created_at, updated_at, last_login_at
	`
	var out user.User
	err := r.db.QueryRow(ctx, q, id, role).Scan(
		&out.ID, &out.Login, &out.Email, &out.PasswordHash, &out.FullName, &out.Phone,
		&out.Role, &out.Status, &out.CreatedAt, &out.UpdatedAt, &out.LastLoginAt,
	)
	if err != nil {
		return nil, user.ErrUserNotFound
	}
	return &out, nil
}

func (r *UserRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status user.Status) (*user.User, error) {
	q := `
		UPDATE users
		SET status = $2
		WHERE id = $1
		RETURNING id, login, email, password_hash, full_name, phone, role, status, created_at, updated_at, last_login_at
	`

	var out user.User
	err := r.db.QueryRow(ctx, q, id, status).Scan(
		&out.ID,
		&out.Login,
		&out.Email,
		&out.PasswordHash,
		&out.FullName,
		&out.Phone,
		&out.Role,
		&out.Status,
		&out.CreatedAt,
		&out.UpdatedAt,
		&out.LastLoginAt,
	)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

func (r *UserRepo) TouchLastLogin(ctx context.Context, id uuid.UUID, at time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET last_login_at = $2 WHERE id = $1`, id, at)
	return err
}

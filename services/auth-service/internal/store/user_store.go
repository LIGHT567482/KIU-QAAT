package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qaat/auth-service/internal/models"
)


var ErrNotFound = errors.New("user not found")
var ErrLocked = errors.New("account locked")

type UserStore struct {
	pool *pgxpool.Pool
}

func NewUserStore(pool *pgxpool.Pool) *UserStore {
	return &UserStore{pool: pool}
}

// GetByEmailAndTenant fetches a user with RLS bypassed (auth service queries
// users table without a tenant filter set — the tenant_id is part of the
// WHERE clause instead, so we use a superuser connection here intentionally).
func (s *UserStore) GetByEmailAndTenant(ctx context.Context, email string, tenantID string) (*models.User, error) {
	const q = `
		SELECT user_id, tenant_id, email, password_hash, role, full_name,
		       COALESCE(title,''), COALESCE(registration_number,''),
		       is_active, totp_secret_enc, totp_enabled, totp_backup_codes_enc,
		       device_binding_key_enc, last_login_at, failed_login_count,
		       locked_until, created_at, updated_at
		FROM users
		WHERE email = $1 AND tenant_id = $2`

	row := s.pool.QueryRow(ctx, q, email, tenantID)
	u := &models.User{}
	var role string
	err := row.Scan(
		&u.UserID, &u.TenantID, &u.Email, &u.PasswordHash, &role, &u.FullName,
		&u.Title, &u.RegistrationNumber,
		&u.IsActive, &u.TOTPSecretEnc, &u.TOTPEnabled, &u.TOTPBackupCodesEnc,
		&u.DeviceBindingKeyEnc, &u.LastLoginAt, &u.FailedLoginCount,
		&u.LockedUntil, &u.CreatedAt, &u.UpdatedAt,
	)
	u.Role = models.Role(role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}
	return u, nil
}

// TenantInstitutionID returns the tenant's institution_id (the ADMIN access code),
// or "" if unset. Used to gate ADMIN logins.
func (s *UserStore) TenantInstitutionID(ctx context.Context, tenantID string) (string, error) {
	var id *string
	err := s.pool.QueryRow(ctx,
		`SELECT institution_id FROM tenants WHERE tenant_id = $1`, tenantID).Scan(&id)
	if err != nil {
		return "", err
	}
	if id == nil {
		return "", nil
	}
	return *id, nil
}

func (s *UserStore) RecordLoginSuccess(ctx context.Context, userID string) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET last_login_at = $1, failed_login_count = 0, locked_until = NULL
		 WHERE user_id = $2`,
		now, userID)
	return err
}

func (s *UserStore) RecordLoginFailure(ctx context.Context, userID string) error {
	// Lock after 5 consecutive failures for 15 minutes.
	_, err := s.pool.Exec(ctx, `
		UPDATE users
		SET failed_login_count = failed_login_count + 1,
		    locked_until = CASE
		        WHEN failed_login_count + 1 >= 5
		        THEN now() + INTERVAL '15 minutes'
		        ELSE locked_until
		    END
		WHERE user_id = $1`, userID)
	return err
}

func (s *UserStore) SetTOTPSecret(ctx context.Context, userID string, encSecret string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET totp_secret_enc = $1, updated_at = now() WHERE user_id = $2`,
		encSecret, userID)
	return err
}

func (s *UserStore) EnableTOTP(ctx context.Context, userID string, encBackupCodes string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET totp_enabled = true, totp_backup_codes_enc = $1, updated_at = now()
		 WHERE user_id = $2`,
		encBackupCodes, userID)
	return err
}

// SetDeviceBindingKey persists the (master-key encrypted) device binding key
// issued to a Coordinator on first login.
func (s *UserStore) SetDeviceBindingKey(ctx context.Context, userID string, encKey string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET device_binding_key_enc = $1, updated_at = now() WHERE user_id = $2`,
		encKey, userID)
	return err
}

// UpdateEmail changes the user's sign-in email (self-service).
func (s *UserStore) UpdateEmail(ctx context.Context, userID string, email string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET email = $1, updated_at = now() WHERE user_id = $2`, email, userID)
	return err
}

// TenantDomain returns the institution's email domain (for domain enforcement).
func (s *UserStore) TenantDomain(ctx context.Context, tenantID string) (string, error) {
	var d string
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(domain,'') FROM tenants WHERE tenant_id = $1`, tenantID).Scan(&d)
	return d, err
}

// UpdatePassword sets a new bcrypt hash for the user (self-service change).
func (s *UserStore) UpdatePassword(ctx context.Context, userID string, passwordHash string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET password_hash = $1, failed_login_count = 0, locked_until = NULL, updated_at = now()
		 WHERE user_id = $2`,
		passwordHash, userID)
	return err
}

func (s *UserStore) GetByID(ctx context.Context, userID string) (*models.User, error) {
	const q = `
		SELECT user_id, tenant_id, email, password_hash, role, full_name,
		       is_active, totp_secret_enc, totp_enabled, totp_backup_codes_enc,
		       device_binding_key_enc, last_login_at, failed_login_count,
		       locked_until, created_at, updated_at
		FROM users WHERE user_id = $1`

	row := s.pool.QueryRow(ctx, q, userID)
	u := &models.User{}
	var role string
	err := row.Scan(
		&u.UserID, &u.TenantID, &u.Email, &u.PasswordHash, &role, &u.FullName,
		&u.IsActive, &u.TOTPSecretEnc, &u.TOTPEnabled, &u.TOTPBackupCodesEnc,
		&u.DeviceBindingKeyEnc, &u.LastLoginAt, &u.FailedLoginCount,
		&u.LockedUntil, &u.CreatedAt, &u.UpdatedAt,
	)
	u.Role = models.Role(role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query user by id: %w", err)
	}
	return u, nil
}

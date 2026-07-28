package models

import "time"

type Role string

const (
	RoleCoordinator Role = "COORDINATOR"
	RoleQAOfficer   Role = "QA_OFFICER"
	RoleDQADirector Role = "DQA_DIRECTOR"
	RoleVC          Role = "VC"
	RoleDVC         Role = "DVC" // Deputy Vice-Chancellor (oversight tier)
	RoleAdmin       Role = "ADMIN"
	RoleSuperAdmin  Role = "SUPER_ADMIN" // platform owner (registers tenants, configures branding)
	RoleStudent     Role = "STUDENT"     // learner; logs in by scanning their personal QR
	RoleLecturer    Role = "LECTURER"    // logs into the lecturer dashboard (read-only attendance)
)

// MFARequired returns true for roles that mandate TOTP.
func (r Role) MFARequired() bool {
	return r == RoleVC || r == RoleDQADirector
}

type User struct {
	UserID              string     `db:"user_id"`
	TenantID            string     `db:"tenant_id"`
	Email               string     `db:"email"`
	PasswordHash        string     `db:"password_hash"`
	Role                Role       `db:"role"`
	FullName            string     `db:"full_name"`
	Title               string     `db:"title"`
	RegistrationNumber  string     `db:"registration_number"`
	IsActive            bool       `db:"is_active"`
	TOTPSecretEnc       *string    `db:"totp_secret_enc"`
	TOTPEnabled         bool       `db:"totp_enabled"`
	TOTPBackupCodesEnc  *string    `db:"totp_backup_codes_enc"`
	DeviceBindingKeyEnc *string    `db:"device_binding_key_enc"`
	LastLoginAt         *time.Time `db:"last_login_at"`
	FailedLoginCount    int16      `db:"failed_login_count"`
	LockedUntil         *time.Time `db:"locked_until"`
	ForcePasswordChange bool       `db:"force_password_change"`
	CreatedAt           time.Time  `db:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at"`
}

func (u *User) IsLocked(now time.Time) bool {
	return u.LockedUntil != nil && now.Before(*u.LockedUntil)
}

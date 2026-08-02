package handlers

// Lecturer + student login provisioning, shared by every passwordless entry point.
//
// Lecturers sign in with (institution, staff ID) — see lecturer_login.go — and students
// by registration number. Both need a backing `users` row so a JWT subject exists; these
// helpers create and link one on demand, and mint the LECTURER token via auth-service.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// ensureLecturerLogin makes sure the lecturer has a linked LECTURER user account
// (so a JWT subject exists), auto-creating one with a synthesised institution email
// if necessary, and returns that user_id.
func ensureLecturerLogin(ctx context.Context, pool *pgxpool.Pool, tenantID, lecturerID string) (string, error) {
	var userID, email, name, domain string
	err := pool.QueryRow(ctx, `
		SELECT COALESCE(l.user_id::text,''), COALESCE(l.email,''), l.full_name, COALESCE(t.domain,'')
		FROM lecturers l JOIN tenants t ON t.tenant_id = l.tenant_id
		WHERE l.lecturer_id = $1::uuid AND l.tenant_id = $2`, lecturerID, tenantID).Scan(&userID, &email, &name, &domain)
	if err != nil {
		return "", fmt.Errorf("lecturer not found")
	}
	if userID != "" {
		return userID, nil
	}
	// Synthesise an email if the lecturer has none (sign-in is by staff ID, not email).
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !emailInDomain(email, strings.ToLower(domain)) {
		short := strings.ReplaceAll(lecturerID, "-", "")
		if len(short) > 10 {
			short = short[:10]
		}
		email = fmt.Sprintf("lecturer.%s@%s", short, domain)
	}
	// Default sign-in password for the unified app; the lecturer changes it later
	// (see migration 052 for rows created before that default existed).
	hash, _ := bcrypt.GenerateFromPassword([]byte(DefaultLecturerPassword), 12)

	if err := pool.QueryRow(ctx, `
		INSERT INTO users (tenant_id, email, password_hash, role, full_name, force_password_change)
		VALUES ($1, $2, $3, 'LECTURER', $4, true)
		ON CONFLICT (tenant_id, email) DO UPDATE SET role = users.role
		RETURNING user_id::text`,
		tenantID, email, string(hash), name).Scan(&userID); err != nil {
		return "", fmt.Errorf("could not create lecturer login: %w", err)
	}
	_, _ = pool.Exec(ctx, `UPDATE lecturers SET user_id = $1::uuid WHERE lecturer_id = $2::uuid AND tenant_id = $3`,
		userID, lecturerID, tenantID)
	return userID, nil
}

// ensureStudentLogin guarantees a STUDENT login account exists for the given (synthetic or real)
// student email, so a student who resolves by registration number can always password-login. This
// mirrors the SIS-import hidden login, covering students imported before that existed or when the
// tenant had no domain set. Best-effort: DefaultStudentPassword, forced change on first sign-in.
func ensureStudentLogin(ctx context.Context, pool *pgxpool.Pool, tenantID, email, fullName string) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return
	}
	var exists bool
	if pool.QueryRow(ctx, `SELECT true FROM users WHERE tenant_id = $1 AND lower(email) = lower($2) LIMIT 1`,
		tenantID, email).Scan(&exists) == nil && exists {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(DefaultStudentPassword), 10)
	if err != nil {
		return
	}
	_, _ = pool.Exec(ctx, `
		INSERT INTO users (tenant_id, email, password_hash, role, full_name, force_password_change)
		VALUES ($1, $2, $3, 'STUDENT', $4, true)
		ON CONFLICT (tenant_id, email) DO NOTHING`,
		tenantID, email, string(hash), fullName)
}

func mintLecturerToken(ctx context.Context, userID, tenantID string) ([]byte, int) {
	base := os.Getenv("AUTH_SERVICE_URL")
	if base == "" {
		base = "http://auth-service:8081"
	}
	body, _ := json.Marshal(map[string]string{"user_id": userID, "tenant_id": tenantID})
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(rctx, http.MethodPost, base+"/internal/lecturer-token", bytes.NewReader(body))
	if err != nil {
		return nil, http.StatusBadGateway
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Key", os.Getenv("INTERNAL_SVC_KEY"))
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, http.StatusBadGateway
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	return buf.Bytes(), resp.StatusCode
}

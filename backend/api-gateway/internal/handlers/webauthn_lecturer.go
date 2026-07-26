package handlers

// Lecturer biometric identity via WebAuthn passkeys (the phone's own fingerprint
// / Face unlock — no external hardware). Two flows, both public (the lecturer has
// no JWT; the enrol token / session_id carry the tenant), run on the adminPool:
//
//   ENROL (once, from an admin-issued link on the lecturer's phone):
//     GET  /lecturer/enroll?tk=<token>              → enrol page
//     POST /api/v1/lecturer/webauthn/enroll/begin    {token}
//     POST /api/v1/lecturer/webauthn/enroll/finish?tk=<token>   (attestation body)
//
//   VERIFY (every time they scan the session QR):
//     POST /api/v1/lecturer/webauthn/assert/begin?s=<session_id>
//     POST /api/v1/lecturer/webauthn/assert/finish?s=<session_id>  (assertion body)
//   …on success a short-lived Redis flag wa:verified:<session_id> is set, which
//   LecturerGateScan requires before recording START/END.
//
// The fingerprint never leaves the device — the OS matches it locally and only a
// signed assertion is sent, which the gateway verifies on the LAN (works offline).

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/qaat/api-gateway/internal/middleware"
)

// getWebAuthn builds the relying-party config from the host the TENANT is using
// (the request) so the passkey RP follows each tenant's own address — SaaS, not a
// single baked domain. WEBAUTHN_RP_ID / WEBAUTHN_RP_ORIGINS are honoured only as
// explicit overrides (single-domain deployments). Not cached, since RP varies by host.
func getWebAuthn(r *http.Request) (*webauthn.WebAuthn, error) {
	base := requestBaseURL(r) // scheme://host derived from the request
	rpID := base
	if i := strings.Index(rpID, "://"); i >= 0 {
		rpID = rpID[i+3:]
	}
	if i := strings.Index(rpID, ":"); i >= 0 {
		rpID = rpID[:i] // strip port → bare hostname (WebAuthn RP ID rule)
	}
	origins := []string{base}

	// Optional explicit overrides for a fixed-domain deployment.
	if v := strings.TrimSpace(os.Getenv("WEBAUTHN_RP_ID")); v != "" {
		rpID = v
	}
	if v := os.Getenv("WEBAUTHN_RP_ORIGINS"); strings.TrimSpace(v) != "" {
		origins = nil
		for _, o := range strings.Split(v, ",") {
			if o = strings.TrimSpace(o); o != "" {
				origins = append(origins, o)
			}
		}
	}
	name := os.Getenv("WEBAUTHN_RP_NAME")
	if name == "" {
		name = "QAAT"
	}
	return webauthn.New(&webauthn.Config{RPDisplayName: name, RPID: rpID, RPOrigins: origins})
}

// waLecturer adapts a lecturer row to the webauthn.User interface.
type waLecturer struct {
	id    string
	name  string
	creds []webauthn.Credential
}

func (l waLecturer) WebAuthnID() []byte                         { return []byte(l.id) }
func (l waLecturer) WebAuthnName() string                       { return l.name }
func (l waLecturer) WebAuthnDisplayName() string                { return l.name }
func (l waLecturer) WebAuthnCredentials() []webauthn.Credential { return l.creds }

func loadLecturerCreds(ctx context.Context, pool *pgxpool.Pool, tenantID, lecturerID string) ([]webauthn.Credential, error) {
	rows, err := pool.Query(ctx,
		`SELECT credential FROM lecturer_webauthn_credentials
		 WHERE tenant_id = $1 AND lecturer_id = $2 AND credential IS NOT NULL`,
		tenantID, lecturerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var creds []webauthn.Credential
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		var c webauthn.Credential
		if json.Unmarshal(raw, &c) == nil {
			creds = append(creds, c)
		}
	}
	return creds, nil
}

func saveLecturerCred(ctx context.Context, pool *pgxpool.Pool, tenantID, lecturerID string, c *webauthn.Credential) error {
	raw, _ := json.Marshal(c)
	credID := base64.RawURLEncoding.EncodeToString(c.ID)
	_, err := pool.Exec(ctx, `
		INSERT INTO lecturer_webauthn_credentials
		    (credential_id, tenant_id, lecturer_id, public_key, attestation_type, aaguid, sign_count, credential)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (credential_id) DO UPDATE
		    SET sign_count = EXCLUDED.sign_count, credential = EXCLUDED.credential, last_used_at = now()`,
		credID, tenantID, lecturerID, c.PublicKey, c.AttestationType, c.Authenticator.AAGUID,
		int64(c.Authenticator.SignCount), raw)
	return err
}

func waStoreSession(ctx context.Context, rdb *redis.Client, key string, s *webauthn.SessionData) {
	b, _ := json.Marshal(s)
	rdb.Set(ctx, key, b, 5*time.Minute)
}
func waLoadSession(ctx context.Context, rdb *redis.Client, key string) (*webauthn.SessionData, bool) {
	b, err := rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	var s webauthn.SessionData
	if json.Unmarshal(b, &s) != nil {
		return nil, false
	}
	return &s, true
}

// resolveEnrollToken maps an admin-issued enrol token → lecturer.
func resolveEnrollToken(ctx context.Context, rdb *redis.Client, pool *pgxpool.Pool, token string) (lecturerID, tenantID, name string, ok bool) {
	if token == "" {
		return "", "", "", false
	}
	val, err := rdb.Get(ctx, "wa:enroll:"+token).Result()
	if err != nil {
		return "", "", "", false
	}
	parts := strings.SplitN(val, "|", 2)
	if len(parts) != 2 {
		return "", "", "", false
	}
	lecturerID, tenantID = parts[0], parts[1]
	_ = pool.QueryRow(ctx, `SELECT full_name FROM lecturers WHERE lecturer_id = $1 AND tenant_id = $2`,
		lecturerID, tenantID).Scan(&name)
	if name == "" {
		name = "Lecturer"
	}
	return lecturerID, tenantID, name, true
}

// ── Enrol ────────────────────────────────────────────────────────────────────

func LecturerEnrollBegin(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Token string `json:"token"` }
		_ = decodeJSON(r, &body)
		lid, tid, name, ok := resolveEnrollToken(r.Context(), rdb, pool, body.Token)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errBody("BAD_TOKEN", "enrolment link is invalid or expired"))
			return
		}
		wa, err := getWebAuthn(r)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("WEBAUTHN_CONFIG", err.Error()))
			return
		}
		creds, _ := loadLecturerCreds(r.Context(), pool, tid, lid)
		user := waLecturer{id: lid, name: name, creds: creds}
		options, session, err := wa.BeginRegistration(user)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("WEBAUTHN_BEGIN", err.Error()))
			return
		}
		waStoreSession(r.Context(), rdb, "wa:reg:"+body.Token, session)
		writeJSON(w, http.StatusOK, options)
	}
}

func LecturerEnrollFinish(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("tk")
		lid, tid, name, ok := resolveEnrollToken(r.Context(), rdb, pool, token)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errBody("BAD_TOKEN", "enrolment link is invalid or expired"))
			return
		}
		session, ok := waLoadSession(r.Context(), rdb, "wa:reg:"+token)
		if !ok {
			writeJSON(w, http.StatusBadRequest, errBody("NO_SESSION", "enrolment not started or expired"))
			return
		}
		wa, err := getWebAuthn(r)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("WEBAUTHN_CONFIG", err.Error()))
			return
		}
		user := waLecturer{id: lid, name: name}
		cred, err := wa.FinishRegistration(user, *session, r)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("WEBAUTHN_VERIFY", err.Error()))
			return
		}
		if err := saveLecturerCred(r.Context(), pool, tid, lid, cred); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("SAVE_FAILED", err.Error()))
			return
		}
		rdb.Del(r.Context(), "wa:reg:"+token, "wa:enroll:"+token) // one-time token consumed
		writeJSON(w, http.StatusOK, map[string]string{"status": "ENROLLED"})
	}
}

// ── Verify (assertion) ───────────────────────────────────────────────────────

// lecturerForSession returns the assigned lecturer + tenant for a session.
func lecturerForSession(ctx context.Context, pool *pgxpool.Pool, sessionID string) (lecturerID, tenantID, name string, ok bool) {
	err := pool.QueryRow(ctx, `
		SELECT s.lecturer_id::text, s.tenant_id::text, COALESCE(l.full_name,'')
		FROM sessions s JOIN lecturers l ON l.lecturer_id::text = s.lecturer_id
		WHERE s.session_id = $1`, sessionID).Scan(&lecturerID, &tenantID, &name)
	if err != nil || lecturerID == "" {
		return "", "", "", false
	}
	return lecturerID, tenantID, name, true
}

func LecturerAssertBegin(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sid := r.URL.Query().Get("s")
		if !middleware.ValidTenantID(sid) {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_SESSION_ID", "bad session id"))
			return
		}
		lid, tid, name, ok := lecturerForSession(r.Context(), pool, sid)
		if !ok {
			writeJSON(w, http.StatusNotFound, errBody("NO_LECTURER", "no lecturer assigned to this session"))
			return
		}
		creds, _ := loadLecturerCreds(r.Context(), pool, tid, lid)
		if len(creds) == 0 {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("NOT_ENROLLED", "this lecturer has not enrolled a fingerprint yet"))
			return
		}
		wa, err := getWebAuthn(r)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("WEBAUTHN_CONFIG", err.Error()))
			return
		}
		user := waLecturer{id: lid, name: name, creds: creds}
		options, session, err := wa.BeginLogin(user)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("WEBAUTHN_BEGIN", err.Error()))
			return
		}
		waStoreSession(r.Context(), rdb, "wa:login:"+sid, session)
		writeJSON(w, http.StatusOK, options)
	}
}

func LecturerAssertFinish(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sid := r.URL.Query().Get("s")
		if !middleware.ValidTenantID(sid) {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_SESSION_ID", "bad session id"))
			return
		}
		lid, tid, name, ok := lecturerForSession(r.Context(), pool, sid)
		if !ok {
			writeJSON(w, http.StatusNotFound, errBody("NO_LECTURER", "no lecturer assigned"))
			return
		}
		session, ok := waLoadSession(r.Context(), rdb, "wa:login:"+sid)
		if !ok {
			writeJSON(w, http.StatusBadRequest, errBody("NO_SESSION", "verification not started or expired"))
			return
		}
		wa, err := getWebAuthn(r)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("WEBAUTHN_CONFIG", err.Error()))
			return
		}
		creds, _ := loadLecturerCreds(r.Context(), pool, tid, lid)
		user := waLecturer{id: lid, name: name, creds: creds}
		cred, err := wa.FinishLogin(user, *session, r)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("WEBAUTHN_VERIFY", err.Error()))
			return
		}
		_ = saveLecturerCred(r.Context(), pool, tid, lid, cred) // persist updated sign count
		// Short-lived proof that this session's lecturer just passed biometric auth.
		rdb.Set(r.Context(), "wa:verified:"+sid, lid, 2*time.Minute)
		rdb.Del(r.Context(), "wa:login:"+sid)
		writeJSON(w, http.StatusOK, map[string]string{"status": "VERIFIED"})
	}
}

// ── Admin: issue an enrolment link for a lecturer ────────────────────────────

func AdminLecturerEnrollLink(adminPool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		lecturerID := chi.URLParam(r, "lecturer_id")
		var exists bool
		if err := adminPool.QueryRow(r.Context(),
			`SELECT true FROM lecturers WHERE lecturer_id = $1 AND tenant_id = $2`,
			lecturerID, tenantID).Scan(&exists); err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "lecturer not found"))
			return
		}
		buf := make([]byte, 24)
		_, _ = rand.Read(buf)
		token := base64.RawURLEncoding.EncodeToString(buf)
		rdb.Set(r.Context(), "wa:enroll:"+token, lecturerID+"|"+tenantID, 24*time.Hour)

		// The enrol page MUST be opened on the WebAuthn origin (RP), else the browser
		// rejects the fingerprint registration (origin/RP-ID mismatch). Prefer the
		// first WEBAUTHN_RP_ORIGINS entry; fall back to the student origin, then host.
		base := strings.TrimSpace(strings.SplitN(os.Getenv("WEBAUTHN_RP_ORIGINS"), ",", 2)[0])
		if base == "" {
			base = strings.TrimRight(os.Getenv("STUDENT_CHECKIN_BASE_URL"), "/")
		}
		if base == "" {
			scheme := "https"
			if r.TLS == nil {
				scheme = "http"
			}
			base = fmt.Sprintf("%s://%s", scheme, r.Host)
		}
		base = strings.TrimRight(base, "/")
		writeJSON(w, http.StatusOK, map[string]string{
			"enroll_url": fmt.Sprintf("%s/lecturer/enroll?tk=%s", base, token),
			"expires_in": "24h",
		})
	}
}

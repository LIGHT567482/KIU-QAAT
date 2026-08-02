package handlers

// The QA patroller's second factor: a PIN set on first sign-in and required at every one after.
//
//	GET  /api/v1/patrol/pin        → {pin_set, locked, locked_until, attempts_left}
//	POST /api/v1/patrol/pin        → set it (first time) or change it (old pin required)
//	POST /api/v1/patrol/pin/verify → open the round
//
// WHY. A patrol tick states that a named lecturer was or was not teaching, and the QA reports
// weigh it against the coordinator's own log precisely because it comes from an independent
// observer. The account password is the weak link in that chain — it is what gets shared "to help
// cover the rounds" — and migration 069's handset binding only stops the account MOVING to another
// phone, not someone else picking up THAT phone. The PIN is the half that closes.
//
// The PIN is never returned, never logged, and never leaves the server in either direction except
// as the value being checked. The only fact the app is ever told is whether one has been set.

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/qaat/api-gateway/internal/middleware"
)

const (
	patrolPINMinLen = 4
	patrolPINMaxLen = 8
	// Ten thousand combinations is not many. The limit exists so an unlocked, stolen handset
	// cannot simply be walked through them; it is deliberately low, and clearing it is an admin
	// action so a real lockout is also a conversation.
	patrolPINMaxAttempts = 5
	patrolPINLockout     = 15 * time.Minute
)

// validPatrolPIN enforces digits-only within the length bounds, and refuses the handful of
// sequences that are not a secret at all. A patroller who picks 1234 has added no factor.
func validPatrolPIN(pin string) error {
	if len(pin) < patrolPINMinLen || len(pin) > patrolPINMaxLen {
		return errors.New("the PIN must be between 4 and 8 digits")
	}
	for _, c := range pin {
		if c < '0' || c > '9' {
			return errors.New("the PIN must be digits only")
		}
	}
	// All-same ("0000") and straight runs ("1234", "4321") are the first things anyone tries.
	same, up, down := true, true, true
	for i := 1; i < len(pin); i++ {
		if pin[i] != pin[0] {
			same = false
		}
		if pin[i] != pin[i-1]+1 {
			up = false
		}
		if pin[i] != pin[i-1]-1 {
			down = false
		}
	}
	if same || up || down {
		return errors.New("that PIN is too easy to guess — avoid repeated or running digits")
	}
	return nil
}

type patrolPINState struct {
	Set          bool   `json:"pin_set"`
	Locked       bool   `json:"locked"`
	LockedUntil  string `json:"locked_until,omitempty"`
	AttemptsLeft int    `json:"attempts_left"`
}

// withPatrolConn runs fn on a connection that has the tenant GUC set.
//
// EVERY query in this file used to go straight at the pool. The patrol tables carry an RLS policy
// of `tenant_id = current_setting('app.current_tenant', true)::uuid`, and on a connection where
// that setting was never applied `current_setting(..., true)` returns the EMPTY STRING — so the
// policy itself evaluates `”::uuid` and the query dies with "invalid input syntax for type uuid".
// Not a permission denial, not an empty result: a 500 on every single call. SetTenant middleware
// does not set the GUC globally, it only stashes a helper in the request context, so a handler that
// wants RLS-scoped access has to do this itself — exactly as BindPatrolDevice next door does.
func withPatrolConn(r *http.Request, pool *pgxpool.Pool, tenantID string, fn func(*pgxpool.Conn) error) error {
	conn, err := pool.Acquire(r.Context())
	if err != nil {
		return err
	}
	defer conn.Release()
	if err := middleware.SetTenantConn(r.Context(), conn, tenantID); err != nil {
		return err
	}
	return fn(conn)
}

// loadPatrolPIN reads the row and normalises an expired lockout away, so every caller sees the
// same truth without each having to compare timestamps.
func loadPatrolPIN(r *http.Request, conn *pgxpool.Conn, tenantID, userID string) (hash string, st patrolPINState, err error) {
	var locked *time.Time
	var attempts int
	err = conn.QueryRow(r.Context(),
		`SELECT pin_hash, failed_attempts, locked_until
		   FROM patroller_pins WHERE tenant_id = $1 AND user_id = $2::uuid`,
		tenantID, userID).Scan(&hash, &attempts, &locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", patrolPINState{Set: false, AttemptsLeft: patrolPINMaxAttempts}, nil
	}
	if err != nil {
		return "", patrolPINState{}, err
	}
	st.Set = true
	if locked != nil && locked.After(time.Now()) {
		st.Locked = true
		st.LockedUntil = locked.Format(time.RFC3339)
	} else {
		// The window has passed: the row may still say "5 failures", but the account is usable.
		attempts = 0
	}
	st.AttemptsLeft = patrolPINMaxAttempts - attempts
	if st.AttemptsLeft < 0 {
		st.AttemptsLeft = 0
	}
	return hash, st, nil
}

// PatrolPINStatus — GET /api/v1/patrol/pin. Tells the app which screen to show: "set your PIN"
// on a first sign-in, "enter your PIN" thereafter, or the lockout.
func PatrolPINStatus(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		var st patrolPINState
		err := withPatrolConn(r, pool, tenantID, func(conn *pgxpool.Conn) error {
			var e error
			_, st, e = loadPatrolPIN(r, conn, tenantID, userID)
			return e
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, st)
	}
}

// SetPatrolPIN — POST /api/v1/patrol/pin {pin, current_pin?}
//
// First time: `pin` alone. Afterwards: `current_pin` must be right too, so a handset left unlocked
// for a minute cannot be used to overwrite the PIN and take the account.
func SetPatrolPIN(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())

		var req struct {
			PIN        string `json:"pin"`
			CurrentPIN string `json:"current_pin"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		req.PIN = strings.TrimSpace(req.PIN)
		req.CurrentPIN = strings.TrimSpace(req.CurrentPIN)
		if err := validPatrolPIN(req.PIN); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("WEAK_PIN", err.Error()))
			return
		}

		var existing string
		var st patrolPINState
		if err := withPatrolConn(r, pool, tenantID, func(conn *pgxpool.Conn) error {
			var e error
			existing, st, e = loadPatrolPIN(r, conn, tenantID, userID)
			return e
		}); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if st.Locked {
			writeJSON(w, http.StatusTooManyRequests, errBody("PIN_LOCKED",
				"Too many wrong PINs. Wait for the lockout to pass, or ask an administrator to reset it."))
			return
		}
		if st.Set {
			if bcrypt.CompareHashAndPassword([]byte(existing), []byte(req.CurrentPIN)) != nil {
				writeJSON(w, http.StatusUnauthorized, errBody("INVALID_PIN", "your current PIN is incorrect"))
				return
			}
			if req.CurrentPIN == req.PIN {
				writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "the new PIN must be different"))
				return
			}
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.PIN), 12)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "could not store the PIN"))
			return
		}
		err = withPatrolConn(r, pool, tenantID, func(conn *pgxpool.Conn) error {
			_, e := conn.Exec(r.Context(), `
				INSERT INTO patroller_pins (user_id, tenant_id, pin_hash, set_at, failed_attempts, locked_until)
				VALUES ($1::uuid, $2, $3, now(), 0, NULL)
				ON CONFLICT (user_id)
				DO UPDATE SET pin_hash = EXCLUDED.pin_hash, set_at = now(),
				              failed_attempts = 0, locked_until = NULL`,
				userID, tenantID, string(hash))
			return e
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "PIN_SET"})
	}
}

// VerifyPatrolPIN — POST /api/v1/patrol/pin/verify {pin}
//
// The gate in front of the round. A wrong PIN counts against the lockout; a right one clears the
// count. The response never distinguishes "no PIN set" from "wrong PIN" by status code alone —
// the app reads `pin_set` from the status endpoint for that, which needs no guessing.
func VerifyPatrolPIN(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())

		var req struct {
			PIN string `json:"pin"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		req.PIN = strings.TrimSpace(req.PIN)

		var hash string
		var st patrolPINState
		if err := withPatrolConn(r, pool, tenantID, func(conn *pgxpool.Conn) error {
			var e error
			hash, st, e = loadPatrolPIN(r, conn, tenantID, userID)
			return e
		}); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if !st.Set {
			writeJSON(w, http.StatusConflict, errBody("PIN_NOT_SET", "set your patrol PIN first"))
			return
		}
		if st.Locked {
			writeJSON(w, http.StatusTooManyRequests, errBody("PIN_LOCKED",
				"Too many wrong PINs. Wait for the lockout to pass, or ask an administrator to reset it."))
			return
		}

		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.PIN)) != nil {
			// Count the failure and lock once the limit is reached. Done in one statement so two
			// racing attempts cannot both read "4" and each write "5".
			var attempts int
			var lockedUntil *time.Time
			_ = withPatrolConn(r, pool, tenantID, func(conn *pgxpool.Conn) error {
				return conn.QueryRow(r.Context(), `
					UPDATE patroller_pins
					   SET failed_attempts = failed_attempts + 1,
					       locked_until = CASE WHEN failed_attempts + 1 >= $3 THEN now() + $4::interval ELSE locked_until END
					 WHERE tenant_id = $1 AND user_id = $2::uuid
					 RETURNING failed_attempts, locked_until`,
					tenantID, userID, patrolPINMaxAttempts, patrolPINLockout.String(),
				).Scan(&attempts, &lockedUntil)
			})

			left := patrolPINMaxAttempts - attempts
			if left < 0 {
				left = 0
			}
			if lockedUntil != nil && lockedUntil.After(time.Now()) {
				writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{
					"error":   "PIN_LOCKED",
					"message": "Too many wrong PINs. Wait for the lockout to pass, or ask an administrator to reset it.",
					// The app counts down against this rather than inventing its own timer.
					"locked_until": lockedUntil.Format(time.RFC3339),
				})
				return
			}
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"error":         "INVALID_PIN",
				"message":       "That PIN is not right.",
				"attempts_left": left,
			})
			return
		}

		_ = withPatrolConn(r, pool, tenantID, func(conn *pgxpool.Conn) error {
			_, e := conn.Exec(r.Context(), `
				UPDATE patroller_pins SET last_verified_at = now(), failed_attempts = 0, locked_until = NULL
				 WHERE tenant_id = $1 AND user_id = $2::uuid`, tenantID, userID)
			return e
		})
		writeJSON(w, http.StatusOK, map[string]string{"status": "VERIFIED"})
	}
}

// ResetPatrolPIN — DELETE /api/v1/admin/patrol-pins/{user_id}
//
// The forgotten-PIN path, and the only way out of a lockout that has not expired. Deleting the row
// puts the patroller back to "set your PIN on next sign-in"; it does NOT set one for them, so an
// administrator never knows a patroller's PIN. Audited, because a reset is exactly the moment
// worth being able to look back at — the same reasoning as releasing a device binding.
func ResetPatrolPIN(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		actorID := middleware.GetUserID(r.Context())
		targetID := strings.TrimSpace(chi.URLParam(r, "user_id"))
		if targetID == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "missing user id"))
			return
		}
		var ct pgconn.CommandTag
		err := withPatrolConn(r, pool, tenantID, func(conn *pgxpool.Conn) error {
			var e error
			ct, e = conn.Exec(r.Context(),
				`DELETE FROM patroller_pins WHERE tenant_id = $1 AND user_id = $2::uuid`, tenantID, targetID)
			return e
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if ct.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "that patroller has no PIN set"))
			return
		}
		auditAdmin(r, pool, tenantID, actorID, "PATROL_PIN_RESET", "user", targetID,
			`{"note":"patrol PIN cleared; the patroller must set a new one on next sign-in"}`)
		writeJSON(w, http.StatusOK, map[string]string{"status": "PIN_RESET"})
	}
}

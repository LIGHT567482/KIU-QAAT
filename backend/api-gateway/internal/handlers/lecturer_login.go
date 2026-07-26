package handlers

// Lecturer dashboard login — PASSWORDLESS (institution + staff ID).
//
// Lecturers are provisioned QR-only: they never hold a usable password (see
// ensureLecturerLogin, which assigns an unusable random one). The admin-console
// "Sign in with your Staff ID" form therefore authenticates the same way the
// read-only lecturer portal does — the (institution, staff_id) pair resolves the
// lecturer within their tenant — and this handler then ensures a LECTURER login
// account exists and asks auth-service to mint a read-only LECTURER token.
//
//   POST /api/v1/auth/lecturer-login   body {"staff_id":"…","org":"…"}
//
// A LECTURER token only grants the two read-only dashboard endpoints
// (/api/v1/lecturer/overview + /attendance), so the trust level matches the
// portal. The route is per-IP rate limited to slow staff-ID enumeration.

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LecturerPasswordlessLogin resolves institution + staff ID → lecturer, ensures a
// LECTURER login account, and returns a minted LECTURER token (same JSON shape as
// /api/v1/auth/login: access_token + role/tenant claims).
func LecturerPasswordlessLogin(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			StaffID string `json:"staff_id"`
			Org     string `json:"org"`
		}
		if err := decodeJSON(r, &req); err != nil ||
			strings.TrimSpace(req.StaffID) == "" || strings.TrimSpace(req.Org) == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "staff_id and org are required"))
			return
		}

		tenantID, ok := portalResolveTenant(r.Context(), adminPool, req.Org)
		if !ok {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "institution not found"))
			return
		}
		lecturerID, _, ok := portalResolveLecturer(r.Context(), adminPool, tenantID, req.StaffID)
		if !ok {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "no lecturer with that staff ID at this institution"))
			return
		}

		userID, err := ensureLecturerLogin(r.Context(), adminPool, tenantID, lecturerID)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("NO_LOGIN", err.Error()))
			return
		}
		tokenResp, code := mintLecturerToken(r.Context(), userID, tenantID)
		if code != http.StatusOK {
			writeJSON(w, http.StatusBadGateway, errBody("TOKEN_FAILED", "could not issue lecturer token"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(tokenResp)
	}
}

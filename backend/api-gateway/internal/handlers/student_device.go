package handlers

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type registerDeviceRequest struct {
	RegNumber         string `json:"reg_number"`
	Org               string `json:"org"`
	DeviceFingerprint string `json:"device_fingerprint"`
}

// RegisterDevice binds a student's phone (a stable fingerprint hash) to their registration number,
// so the native student app enforces one-device-one-student GLOBALLY at one-time onboarding.
//
// Endpoint: POST /api/v1/student/register-device  (public — students have no JWT)
// Body:     {reg_number, org, device_fingerprint}
//
// The org slug resolves the tenant (like the reg-no progress portal), then the reg must be an
// ACTIVE student of that tenant. Idempotent for the same device; rejects a device already tied to
// another student, or a student already onboarded on another device. Uses the privileged adminPool
// and self-scopes by tenant_id (no RLS). The per-LECTURE device lock is unchanged and offline.
func RegisterDevice(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerDeviceRequest
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "reg_number, org and device_fingerprint are required"))
			return
		}
		reg := strings.TrimSpace(req.RegNumber)
		org := strings.TrimSpace(req.Org)
		fp := strings.TrimSpace(req.DeviceFingerprint)
		if reg == "" || org == "" || fp == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "reg_number, org and device_fingerprint are required"))
			return
		}
		ctx := r.Context()

		// Resolve the tenant from the institution slug (domain or institution_id).
		var tenantID string
		if err := adminPool.QueryRow(ctx,
			`SELECT tenant_id::text FROM tenants WHERE lower(domain) = lower($1) OR lower(institution_id) = lower($1) LIMIT 1`,
			org).Scan(&tenantID); err != nil || tenantID == "" {
			writeJSON(w, http.StatusNotFound, errBody("TENANT_NOT_FOUND", "we couldn't find that institution — check the institution ID"))
			return
		}

		// The reg number must be an ACTIVE student of that tenant.
		var fullName string
		if err := adminPool.QueryRow(ctx,
			`SELECT full_name FROM students_extended
			  WHERE student_id = $1 AND tenant_id = $2 AND enrollment_status = 'ACTIVE'`,
			reg, tenantID).Scan(&fullName); err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "no active student with that registration number at this institution"))
			return
		}

		// (a) Is this device already bound to a DIFFERENT student?
		var deviceOwner string
		if err := adminPool.QueryRow(ctx,
			`SELECT student_id FROM student_device_bindings WHERE device_fingerprint_hash = $1`,
			fp).Scan(&deviceOwner); err == nil && deviceOwner != reg {
			writeJSON(w, http.StatusConflict, errBody("DEVICE_TAKEN", "this phone is already registered to another student"))
			return
		}
		// (b) Is this student already bound to a DIFFERENT device?
		var studentDevice string
		if err := adminPool.QueryRow(ctx,
			`SELECT device_fingerprint_hash FROM student_device_bindings WHERE student_id = $1`,
			reg).Scan(&studentDevice); err == nil && studentDevice != fp {
			writeJSON(w, http.StatusConflict, errBody("STUDENT_ON_ANOTHER_DEVICE", "your registration is already set up on another phone — ask your admin to reset it"))
			return
		}

		// Upsert — idempotent when the same student re-onboards on the same device.
		if _, err := adminPool.Exec(ctx,
			`INSERT INTO student_device_bindings (student_id, tenant_id, device_fingerprint_hash)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (student_id)
			 DO UPDATE SET device_fingerprint_hash = EXCLUDED.device_fingerprint_hash, updated_at = now()`,
			reg, tenantID, fp); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "could not register this device"))
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"student_id": reg, "full_name": fullName})
	}
}

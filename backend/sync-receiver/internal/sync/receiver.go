package sync

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/qaat/sync-receiver/internal/auth"
	"github.com/qaat/sync-receiver/internal/crypto"
)

// tenantIDPattern matches a canonical UUID. The gateway authenticates the JWT
// and forwards the tenant in X-Tenant-ID; we still validate its shape so a
// malformed/garbage value can never reach a query or be stored.
var tenantIDPattern = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// requireTenant returns the caller's tenant from the verified JWT claim that the
// auth middleware placed in the context. It falls back to the X-Tenant-ID header
// only when no verified claim is present (e.g. behind a gateway that strips the
// token), and always validates the UUID shape before use.
func requireTenant(w http.ResponseWriter, r *http.Request) (string, bool) {
	tenantID := auth.TenantID(r.Context())
	if tenantID == "" {
		tenantID = r.Header.Get("X-Tenant-ID")
	}
	if !tenantIDPattern.MatchString(tenantID) {
		writeErr(w, http.StatusForbidden, "INVALID_TENANT", "missing or malformed tenant context")
		return "", false
	}
	return tenantID, true
}

// uploadTenant returns the tenant that owns an upload. Callers compare it to the
// request tenant so one tenant cannot manipulate another tenant's upload.
func uploadTenant(ctx context.Context, pool *pgxpool.Pool, uploadID string) (string, error) {
	var owner string
	err := pool.QueryRow(ctx,
		`SELECT tenant_id::text FROM sync_uploads WHERE upload_id = $1`, uploadID).Scan(&owner)
	return owner, err
}

// ─── Init ─────────────────────────────────────────────────────────────────────
// POST /api/v1/sync/init

type initRequest struct {
	CoordinatorID   string   `json:"coordinator_id"`
	SessionIDs      []string `json:"session_ids"`
	TotalChunks     int      `json:"total_chunks"`
	PackageChecksum string   `json:"package_checksum"`
	PackageHMAC     string   `json:"package_hmac"`
}

type initResponse struct {
	UploadID        string `json:"upload_id"`
	ChunkSizeBytes  int    `json:"chunk_size_bytes"`
	ServerTimestamp string `json:"server_timestamp"`
}

const chunkSize = 65536 // 64 KiB

// maxChunks caps a single upload at ~640 MiB and bounds the chunk_index space,
// preventing unbounded Redis-key creation from a malicious total_chunks/index.
const maxChunks = 10000

func Init(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := requireTenant(w, r)
		if !ok {
			return
		}

		var req initRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "malformed body")
			return
		}
		if req.TotalChunks <= 0 || req.TotalChunks > maxChunks {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "total_chunks out of range")
			return
		}

		// Bind the upload to the authenticated user, not the client-supplied id —
		// the coordinator_id is later used to locate the device key for decryption.
		coordinatorID := auth.UserID(r.Context())
		if coordinatorID == "" {
			coordinatorID = req.CoordinatorID
		}

		var uploadID string
		err := pool.QueryRow(r.Context(), `
			INSERT INTO sync_uploads
			  (tenant_id, coordinator_id, session_ids, total_chunks, package_checksum, package_hmac, status)
			VALUES ($1, $2, $3, $4, $5, $6, 'PENDING')
			RETURNING upload_id::text`,
			tenantID, coordinatorID, req.SessionIDs, req.TotalChunks, req.PackageChecksum, req.PackageHMAC,
		).Scan(&uploadID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not create upload record")
			return
		}

		writeJSON(w, http.StatusOK, initResponse{
			UploadID:        uploadID,
			ChunkSizeBytes:  chunkSize,
			ServerTimestamp: time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// ─── Chunk ───────────────────────────────────────────────────────────────────
// POST /api/v1/sync/chunk/{upload_id}/{chunk_index}
// Body: raw AES-256 encrypted binary chunk

func Chunk(pool *pgxpool.Pool, rdb *redis.Client, serverSignKey []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := requireTenant(w, r)
		if !ok {
			return
		}

		uploadID   := chi.URLParam(r, "upload_id")
		chunkIdxStr := chi.URLParam(r, "chunk_index")
		chunkIdx, err := strconv.Atoi(chunkIdxStr)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "chunk_index must be integer")
			return
		}

		// Verify the upload exists and belongs to the caller's tenant, and that
		// the chunk index is within the declared range. This stops one tenant
		// from poisoning another's upload and bounds the Redis key space.
		var owner string
		var totalChunks int
		if err := pool.QueryRow(r.Context(),
			`SELECT tenant_id::text, total_chunks FROM sync_uploads WHERE upload_id = $1`,
			uploadID).Scan(&owner, &totalChunks); err != nil {
			writeErr(w, http.StatusNotFound, "UPLOAD_NOT_FOUND", "upload id not found")
			return
		}
		if owner != tenantID {
			writeErr(w, http.StatusForbidden, "FORBIDDEN", "upload belongs to another tenant")
			return
		}
		if chunkIdx < 0 || chunkIdx >= totalChunks {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "chunk_index out of range")
			return
		}

		// Read raw chunk with a hard 1 MiB cap; surface read errors instead of
		// silently storing a truncated chunk.
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "could not read chunk body")
			return
		}

		// Store chunk in Redis (TTL 7 days — matches 7-day archive policy).
		chunkKey := fmt.Sprintf("sync:chunk:%s:%d", uploadID, chunkIdx)
		rdb.Set(r.Context(), chunkKey, body, 7*24*time.Hour) //nolint:errcheck

		// Mark chunk received in DB. array_append only when the index is not
		// already present, so resending a chunk cannot inflate received_chunks
		// past total_chunks and fake completeness.
		if _, err := pool.Exec(r.Context(), `
			UPDATE sync_uploads
			SET received_chunks = CASE WHEN $1 = ANY(received_chunks)
			                           THEN received_chunks
			                           ELSE array_append(received_chunks, $1) END,
			    status = 'UPLOADING'
			WHERE upload_id = $2`, chunkIdx, uploadID); err != nil {
			writeErr(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not record chunk")
			return
		}

		// Server ACK: sign "upload_id:chunk_index" with HMAC-SHA256.
		ackPayload := fmt.Sprintf("%s:%d", uploadID, chunkIdx)
		ackSig := hmacSHA256(serverSignKey, ackPayload)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"upload_id":          uploadID,
			"chunk_index":        chunkIdx,
			"next_expected":      chunkIdx + 1,
			"server_ack_signature": ackSig,
		})
	}
}

// ─── Resume ──────────────────────────────────────────────────────────────────
// GET /api/v1/sync/resume/{upload_id}

func Resume(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := requireTenant(w, r)
		if !ok {
			return
		}

		uploadID := chi.URLParam(r, "upload_id")

		var received []int
		var totalChunks int
		var owner string
		err := pool.QueryRow(r.Context(),
			`SELECT received_chunks, total_chunks, tenant_id::text FROM sync_uploads WHERE upload_id = $1`,
			uploadID).Scan(&received, &totalChunks, &owner)
		if err != nil {
			writeErr(w, http.StatusNotFound, "UPLOAD_NOT_FOUND", "upload id not found")
			return
		}
		if owner != tenantID {
			writeErr(w, http.StatusForbidden, "FORBIDDEN", "upload belongs to another tenant")
			return
		}

		// Determine first missing chunk index.
		received_set := make(map[int]bool, len(received))
		for _, c := range received {
			received_set[c] = true
		}
		resumeFrom := 0
		for i := 0; i < totalChunks; i++ {
			if !received_set[i] {
				resumeFrom = i
				break
			}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"upload_id":        uploadID,
			"resume_from_chunk": resumeFrom,
			"received_chunks":  received,
		})
	}
}

// ─── Complete ─────────────────────────────────────────────────────────────────
// POST /api/v1/sync/complete/{upload_id}
// Assembles chunks → validates checksum → writes attendance_logs → triggers eligibility.

func Complete(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqTenant, ok := requireTenant(w, r)
		if !ok {
			return
		}

		uploadID := chi.URLParam(r, "upload_id")

		var received []int
		var totalChunks int
		var coordID, checksum, tenantID string
		var packageHMAC *string
		var sessionIDs []string

		err := pool.QueryRow(r.Context(), `
			SELECT coordinator_id, session_ids, total_chunks, received_chunks,
			       package_checksum, package_hmac, tenant_id::text
			FROM sync_uploads WHERE upload_id = $1`, uploadID).
			Scan(&coordID, &sessionIDs, &totalChunks, &received, &checksum, &packageHMAC, &tenantID)
		if err != nil {
			writeErr(w, http.StatusNotFound, "UPLOAD_NOT_FOUND", "upload id not found")
			return
		}
		if tenantID != reqTenant {
			writeErr(w, http.StatusForbidden, "FORBIDDEN", "upload belongs to another tenant")
			return
		}

		if len(received) < totalChunks {
			writeErr(w, http.StatusConflict, "UPLOAD_INCOMPLETE",
				fmt.Sprintf("received %d of %d chunks", len(received), totalChunks))
			return
		}

		// Reassemble chunks from Redis.
		var fullPayload []byte
		for i := 0; i < totalChunks; i++ {
			chunk, err := rdb.Get(r.Context(), fmt.Sprintf("sync:chunk:%s:%d", uploadID, i)).Bytes()
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "CHUNK_MISSING",
					fmt.Sprintf("chunk %d not found in store", i))
				return
			}
			fullPayload = append(fullPayload, chunk...)
		}

		// Verify package checksum (transport integrity).
		h := sha256.Sum256(fullPayload)
		actualChecksum := hex.EncodeToString(h[:])
		if actualChecksum != checksum {
			writeErr(w, http.StatusUnprocessableEntity, "CHECKSUM_MISMATCH", "package integrity check failed")
			return
		}

		// Authenticate + decrypt the package using the coordinator's device key.
		plaintext, err := decryptPackage(r.Context(), pool, coordID, tenantID, packageHMAC, fullPayload)
		if err != nil {
			writeErr(w, http.StatusUnprocessableEntity, "PACKAGE_UNVERIFIED", err.Error())
			return
		}

		written, duplicates, rejectedNoLecturer, err := writeAttendanceLogs(r.Context(), pool, plaintext, tenantID, coordID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "WRITE_ERROR", err.Error())
			return
		}

		// Mark upload complete.
		pool.Exec(r.Context(), //nolint:errcheck
			`UPDATE sync_uploads SET status = 'SYNCED', completed_at = now() WHERE upload_id = $1`,
			uploadID)

		// Trigger materialized view refresh asynchronously.
		go refreshEligibilityView(context.Background(), pool, tenantID)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":                "SYNCED",
			"records_written":       written,
			"duplicates_rejected":   duplicates,
			"rejected_no_lecturer":  rejectedNoLecturer,
			"sync_timestamp":        time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// decryptPackage loads the coordinator's device binding key, then authenticates
// and decrypts the uploaded payload into the plaintext session-package JSON.
// A missing HMAC, an unknown coordinator, or a failed verification are all hard
// errors — the upload is never marked SYNCED on an unverifiable package.
func decryptPackage(ctx context.Context, pool *pgxpool.Pool, coordinatorID, tenantID string, packageHMAC *string, payload []byte) ([]byte, error) {
	if packageHMAC == nil || *packageHMAC == "" {
		return nil, fmt.Errorf("package is missing its authentication HMAC")
	}

	var bindingKeyEnc *string
	err := pool.QueryRow(ctx,
		`SELECT device_binding_key_enc FROM users WHERE user_id = $1 AND tenant_id = $2`,
		coordinatorID, tenantID).Scan(&bindingKeyEnc)
	if err != nil {
		return nil, fmt.Errorf("coordinator device key not found")
	}
	if bindingKeyEnc == nil || *bindingKeyEnc == "" {
		return nil, fmt.Errorf("coordinator has no device binding key")
	}

	bindingKey, err := crypto.DecryptBindingKey(*bindingKeyEnc, coordinatorID)
	if err != nil {
		return nil, fmt.Errorf("could not unwrap device key: %w", err)
	}

	return crypto.VerifyAndDecryptPackage(bindingKey, payload, *packageHMAC)
}

// writeAttendanceLogs deserialises the decrypted payload and inserts records
// with Vector Clock deduplication (tenant + session + coordinator + sequence).
//
// All inserts run on a single connection that has app.current_tenant set, so
// PostgreSQL RLS (WITH CHECK) is active for the write path too — the receiver is
// never relying on a BYPASSRLS role. The coordinator_id is taken from the
// verified JWT subject, not from the (forgeable) record body, so it anchors the
// vector clock to the device that actually authenticated.
func writeAttendanceLogs(ctx context.Context, pool *pgxpool.Pool, payload []byte, tenantID, coordinatorID string) (written, duplicates, rejectedNoLecturer int, err error) {
	// The outer wrapper is a JSON object produced by sealSessionPackage in the PWA.
	var pkg struct {
		Session struct {
			SessionID     string `json:"session_id"`
			UnitID        string `json:"unit_id"`
			SessionDate   string `json:"session_date"`
			SessionStatus string `json:"session_status"` // "CLOSED" or "AUTO_CLOSED"
		} `json:"session"`
		AttendanceRecords []struct {
			LogID                string `json:"log_id"`
			SessionID            string `json:"session_id"`
			StudentIDHash        string `json:"student_id_hash"`
			DeviceFingerprintHash string `json:"device_fingerprint_hash"`
			SequenceNumber       int    `json:"sequence_number"`
			CheckinTimestamp     string `json:"checkin_timestamp"`
			EntryMethod          string `json:"entry_method"`
		} `json:"attendance_records"`
		// Phone-hub only: the lecturer's physical-presence proof (they scanned the gate to START,
		// and optionally to END). We seed lecturer_attendance_logs from this — which both shows the
		// lecturer's attendance in the dashboards AND makes this session's student attendance
		// "verified" (see the LECTURER-SCAN GATE below).
		Lecturer struct {
			LecturerID         string `json:"lecturer_id"`
			ScannedAt          string `json:"scanned_at"`
			FingerprintHash    string `json:"fingerprint_hash"`
			EndedAt            string `json:"ended_at"`
			EndFingerprintHash string `json:"end_fingerprint_hash"`
		} `json:"lecturer"`
	}
	if err := json.Unmarshal(payload, &pkg); err != nil {
		// Do NOT silently report success. If the payload can't be parsed (e.g.
		// it is corrupt/forged), surface a hard error so the upload is not marked
		// SYNCED with zero records written.
		return 0, 0, 0, fmt.Errorf("payload could not be parsed as an attendance package: %w", err)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	// Activate RLS for this tenant on the write connection. set_config with
	// is_local=false persists across the per-statement implicit transactions pgx
	// issues; this connection is released (and the pool clears the GUC) afterward.
	if _, err := conn.Exec(ctx, "SELECT set_config('app.current_tenant', $1, false)", tenantID); err != nil {
		return 0, 0, 0, fmt.Errorf("set tenant: %w", err)
	}

	// Phone-hub: the session was opened OFFLINE on the device, so its session_id does
	// not exist centrally yet — but attendance_logs FK-references sessions. Create it
	// from the package before inserting rows. Laptop-hub sessions already exist, so
	// ON CONFLICT is a no-op. (No-op too if the package omits the session block.)
	if pkg.Session.SessionID != "" && pkg.Session.UnitID != "" {
		// Preserve how the session ended: AUTO_CLOSED when the coordinator auto-closed it after the
		// scheduled duration + grace; CLOSED otherwise. Whitelist to avoid an invalid enum value.
		sessionStatus := "CLOSED"
		if pkg.Session.SessionStatus == "AUTO_CLOSED" {
			sessionStatus = "AUTO_CLOSED"
		}
		if _, err := conn.Exec(ctx, `
			INSERT INTO sessions (session_id, tenant_id, coordinator_id, unit_id, session_date, session_status)
			VALUES ($1, $2, $3, $4, COALESCE(NULLIF($5,'')::date, CURRENT_DATE), $6::session_status_enum)
			ON CONFLICT (session_id) DO NOTHING`,
			pkg.Session.SessionID, tenantID, coordinatorID, pkg.Session.UnitID, pkg.Session.SessionDate, sessionStatus,
		); err != nil {
			return 0, 0, 0, fmt.Errorf("create session from package: %w", err)
		}
	}

	// Phone-hub: seed lecturer_attendance_logs from the lecturer's offline gate scan, so their
	// attendance shows in the dashboards and the session counts as "verified" below. Resolve the
	// staff ID to the internal lecturer_id when possible (so the dashboard name/dept join works);
	// gate_open_time = the START scan; contact_hours are derived from START→END when END is present.
	// Idempotent (WHERE NOT EXISTS) so a re-upload doesn't duplicate the row.
	if pkg.Lecturer.LecturerID != "" && pkg.Lecturer.ScannedAt != "" &&
		pkg.Session.SessionID != "" && pkg.Session.UnitID != "" {
		if _, lerr := conn.Exec(ctx, `
			INSERT INTO lecturer_attendance_logs
			  (tenant_id, session_id, lecturer_id, gate_open_time, unit_id, session_date,
			   lecturer_scanned_at, lecturer_fingerprint_hash,
			   lecturer_ended_at, lecturer_end_fingerprint_hash, gate_close_time, contact_hours)
			SELECT $1, $2,
			       COALESCE((SELECT lecturer_id::text FROM lecturers WHERE tenant_id = $1 AND staff_id = $3), $3),
			       $4::timestamptz, $5, COALESCE(NULLIF($6,'')::date, CURRENT_DATE),
			       $4::timestamptz, NULLIF($7,''),
			       NULLIF($8,'')::timestamptz, NULLIF($9,''), NULLIF($8,'')::timestamptz,
			       CASE WHEN $8 <> '' THEN ROUND(EXTRACT(EPOCH FROM ($8::timestamptz - $4::timestamptz)) / 3600.0, 2) END
			WHERE NOT EXISTS (
			   SELECT 1 FROM lecturer_attendance_logs WHERE tenant_id = $1 AND session_id = $2)`,
			tenantID, pkg.Session.SessionID, pkg.Lecturer.LecturerID, pkg.Lecturer.ScannedAt,
			pkg.Session.UnitID, pkg.Session.SessionDate, pkg.Lecturer.FingerprintHash,
			pkg.Lecturer.EndedAt, pkg.Lecturer.EndFingerprintHash,
		); lerr != nil {
			return 0, 0, 0, fmt.Errorf("seed lecturer attendance: %w", lerr)
		}
	}

	// The OFFLINE edge carries the privacy-preserving student_id_hash (HMAC-SHA256 of
	// the reg-no, 64 hex chars). The ONLINE check-in stores the raw reg-no. Resolve the
	// hash back to the reg-no here so attendance_logs.student_id is ALWAYS the reg-no —
	// consistent across paths (so they dedup) and within varchar(50).
	hashToReg, err := buildHashIndex(ctx, conn, tenantID)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("build student hash index: %w", err)
	}
	unresolved := 0

	// ── LECTURER-SCAN GATE ────────────────────────────────────────────────────
	// Attendance is persisted ONLY for sessions where the assigned lecturer actually
	// scanned the coordinator's QR (lecturer_attendance_logs.lecturer_scanned_at set).
	// Records for a session with no lecturer scan are NOT written — the lecture is
	// unverified, so its attendance is not valid. (TODO phone-hub: when the offline
	// package carries the lecturer's scan, also seed lecturer_attendance_logs here.)
	sessionSet := map[string]struct{}{}
	if pkg.Session.SessionID != "" {
		sessionSet[pkg.Session.SessionID] = struct{}{}
	}
	for _, rec := range pkg.AttendanceRecords {
		if rec.SessionID != "" {
			sessionSet[rec.SessionID] = struct{}{}
		}
	}
	sessIDs := make([]string, 0, len(sessionSet))
	for id := range sessionSet {
		sessIDs = append(sessIDs, id)
	}
	verified := map[string]bool{}
	if len(sessIDs) > 0 {
		vRows, vErr := conn.Query(ctx, `
			SELECT session_id::text FROM lecturer_attendance_logs
			WHERE tenant_id = $1 AND session_id = ANY($2) AND lecturer_scanned_at IS NOT NULL`,
			tenantID, sessIDs)
		if vErr != nil {
			return 0, 0, 0, fmt.Errorf("check lecturer scans: %w", vErr)
		}
		for vRows.Next() {
			var s string
			_ = vRows.Scan(&s)
			verified[s] = true
		}
		vRows.Close()
	}

	for _, rec := range pkg.AttendanceRecords {
		// TESTING PHASE: the lecturer-scan verification gate is RELAXED. Previously, attendance for a
		// session with no lecturer scan was DROPPED here (continue), which is why a student who
		// attended could still see 0% — their record never reached the summary. We now RECORD every
		// captured attendance so it counts toward progress; lecturer presence is still seeded into
		// lecturer_attendance_logs above when the package carries it. The counter is kept for
		// telemetry only. Re-tighten (restore `continue`) alongside re-enabling the device-lock
		// anti-cheat (AppState.ENFORCE_DEVICE_LOCK) before go-live.
		if !verified[rec.SessionID] {
			rejectedNoLecturer++
		}
		studentID := hashToReg[rec.StudentIDHash]
		if studentID == "" {
			// Fallback: a value that already fits the column is treated as a raw
			// reg-no (back-compat); anything else is an unknown hash → skip (the edge
			// validator already gates on roster membership, so this should be rare).
			if len(rec.StudentIDHash) <= 50 {
				studentID = rec.StudentIDHash
			} else {
				unresolved++
				continue
			}
		}
		tag, execErr := conn.Exec(ctx, `
			INSERT INTO attendance_logs
			  (log_id, tenant_id, session_id, student_id, checkin_timestamp,
			   device_fingerprint_hash, sequence_number, entry_method,
			   coordinator_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (tenant_id, session_id, coordinator_id, sequence_number)
			  WHERE entry_method = 'QR_SCAN' DO NOTHING`,
			rec.LogID, tenantID, rec.SessionID, studentID,
			rec.CheckinTimestamp, rec.DeviceFingerprintHash,
			rec.SequenceNumber, rec.EntryMethod, coordinatorID,
		)
		if execErr != nil {
			// A genuine write failure aborts the batch rather than being
			// miscounted as a duplicate.
			return written, duplicates, rejectedNoLecturer, fmt.Errorf("insert log %s: %w", rec.LogID, execErr)
		}
		// ON CONFLICT DO NOTHING returns no error; 0 rows affected means the
		// vector clock already existed → a real duplicate.
		if tag.RowsAffected() == 0 {
			duplicates++
		} else {
			written++
		}
	}
	if unresolved > 0 {
		slog.Warn("sync: attendance records with unresolved student hashes were skipped",
			"count", unresolved, "tenant", tenantID)
	}
	return written, duplicates, rejectedNoLecturer, nil
}

// buildHashIndex maps HMAC-SHA256(student_hash_key, reg_no) → reg_no for the tenant,
// so the offline path's student_id_hash resolves back to the registration number.
// Runs on the RLS-scoped connection, so it only sees this tenant's students.
func buildHashIndex(ctx context.Context, conn *pgxpool.Conn, tenantID string) (map[string]string, error) {
	var hashKey string
	if err := conn.QueryRow(ctx,
		`SELECT COALESCE(student_hash_key,'') FROM tenants WHERE tenant_id = $1`, tenantID).Scan(&hashKey); err != nil {
		return nil, fmt.Errorf("load hash key: %w", err)
	}
	rows, err := conn.Query(ctx, `SELECT student_id FROM students_extended WHERE tenant_id = $1`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("load students: %w", err)
	}
	defer rows.Close()
	idx := make(map[string]string)
	for rows.Next() {
		var reg string
		if err := rows.Scan(&reg); err != nil {
			return nil, err
		}
		idx[hmacSHA256([]byte(hashKey), reg)] = reg
	}
	return idx, rows.Err()
}

// refreshEligibilityView recomputes the attendance summary for one tenant only,
// via a SECURITY DEFINER function — no global REFRESH MATERIALIZED VIEW lock, so
// one tenant's sync never serialises behind every other tenant's.
func refreshEligibilityView(ctx context.Context, pool *pgxpool.Pool, tenantID string) {
	pool.Exec(ctx, `SELECT refresh_attendance_summary($1)`, tenantID) //nolint:errcheck
}

func hmacSHA256(key []byte, data string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": code, "message": msg}) //nolint:errcheck
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body) //nolint:errcheck
}

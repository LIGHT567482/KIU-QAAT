package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
	"github.com/qaat/api-gateway/internal/upanel"
)

// UPanelAttendance fetches student, lecturer and admin attendance from Contabo U-Panel,
// stores every row in QAAT, and returns the stored copy. Admin campus presence is also
// written into employee_attendance_logs (source=UPANEL) so the employee report uses it.
func UPanelAttendance(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("kind")))
		switch kind {
		case "", upanel.KindStudent, upanel.KindLecturer, upanel.KindAdmin:
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":   "INVALID_KIND",
				"message": "kind must be student, lecturer or admin",
			})
			return
		}

		payload, err := upanel.Fetch(r.Context())
		if err != nil {
			if stored := loadStoredUPanel(r.Context(), pool, kind); len(stored.Records) > 0 {
				stored.Message = "U-Panel unreachable; showing last stored records. " + err.Error()
				writeJSON(w, http.StatusOK, stored)
				return
			}
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":   "UPANEL_UNREACHABLE",
				"message": err.Error(),
			})
			return
		}
		if !payload.Configured {
			if stored := loadStoredUPanel(r.Context(), pool, kind); len(stored.Records) > 0 {
				stored.Message = payload.Message
				writeJSON(w, http.StatusOK, stored)
				return
			}
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error":   "UPANEL_NOT_CONFIGURED",
				"message": payload.Message,
			})
			return
		}

		if n, uerr := upanel.Upsert(r.Context(), pool, payload.Records); uerr == nil {
			payload.Stored = n
			if n > 0 {
				auditAdmin(r, pool, tenantOf(r), middleware.GetUserID(r.Context()), "UPANEL_SYNC", "upanel_attendance", "",
					jsonObject(map[string]string{
						"stored":    strconv.Itoa(n),
						"students":  strconv.Itoa(payload.StudentCount),
						"lecturers": strconv.Itoa(payload.LecturerCount),
						"admins":    strconv.Itoa(payload.AdminCount),
						"via":       payload.FetchedVia,
					}))
			}
		}
		ingestAdminPunches(r.Context(), pool, tenantOf(r), payload.Records)

		if stored, lerr := upanel.List(r.Context(), pool, kind); lerr == nil && stored != nil {
			payload.Records = stored
			payload.RecordCount = len(stored)
			payload.StudentCount = countUPanelKind(stored, upanel.KindStudent)
			payload.LecturerCount = countUPanelKind(stored, upanel.KindLecturer)
			payload.AdminCount = countUPanelKind(stored, upanel.KindAdmin)
		} else if kind != "" {
			payload.Records = filterUPanelKind(payload.Records, kind)
			payload.RecordCount = len(payload.Records)
		}
		writeJSON(w, http.StatusOK, payload)
	}
}

func loadStoredUPanel(ctx context.Context, pool *pgxpool.Pool, kind string) upanel.Payload {
	rows, err := upanel.List(ctx, pool, kind)
	if err != nil || len(rows) == 0 {
		return upanel.Payload{Source: "u-panel", Records: []upanel.Record{}}
	}
	return upanel.Payload{
		Source:        "u-panel",
		Configured:    true,
		FromCache:     true,
		Records:       rows,
		RecordCount:   len(rows),
		StudentCount:  countUPanelKind(rows, upanel.KindStudent),
		LecturerCount: countUPanelKind(rows, upanel.KindLecturer),
		AdminCount:    countUPanelKind(rows, upanel.KindAdmin),
		FetchedVia:    "stored",
	}
}

func ingestAdminPunches(ctx context.Context, pool *pgxpool.Pool, tenantID string, rows []upanel.Record) {
	if pool == nil || tenantID == "" {
		return
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return
	}
	defer conn.Release()
	if err := middleware.SetTenantConn(ctx, conn, tenantID); err != nil {
		return
	}
	for _, rec := range rows {
		if rec.Kind != upanel.KindAdmin {
			continue
		}
		staffID := strings.TrimSpace(rec.StaffID)
		if staffID == "" {
			staffID = strings.TrimSpace(rec.PersonID)
		}
		when, ok := parsePunchTime(rec.Timestamp, "", "")
		if staffID == "" || !ok {
			continue
		}
		name := strings.TrimSpace(rec.FullName)
		if name == "" {
			name = strings.TrimSpace(rec.PersonName)
		}
		if name == "" {
			name = staffID
		}
		title := strings.TrimSpace(rec.Unit)
		if title == "" {
			title = strings.TrimSpace(rec.Course)
		}
		evt := normalizeEventType(rec.EventType)
		_, _ = conn.Exec(ctx, `
			INSERT INTO employees (tenant_id, staff_id, full_name, job_title)
			VALUES ($1,$2,$3,NULLIF($4,''))
			ON CONFLICT (tenant_id, staff_id) DO UPDATE SET
			    full_name = CASE WHEN EXCLUDED.full_name <> '' AND employees.full_name = employees.staff_id
			                     THEN EXCLUDED.full_name ELSE employees.full_name END,
			    job_title = COALESCE(employees.job_title, NULLIF(EXCLUDED.job_title,''))`,
			tenantID, staffID, name, title)
		_, _ = conn.Exec(ctx, `
			INSERT INTO employee_attendance_logs (staff_id, event_time, event_type, source, comment)
			VALUES ($1,$2,$3,'UPANEL',NULLIF($4,''))
			ON CONFLICT (staff_id, event_time, event_type) DO NOTHING`,
			staffID, when.UTC(), evt, "U-Panel campus presence")
	}
}

func filterUPanelKind(rows []upanel.Record, kind string) []upanel.Record {
	out := make([]upanel.Record, 0, len(rows))
	for _, rec := range rows {
		if rec.Kind == kind {
			out = append(out, rec)
		}
	}
	return out
}

func countUPanelKind(rows []upanel.Record, kind string) int {
	n := 0
	for _, rec := range rows {
		if rec.Kind == kind {
			n++
		}
	}
	return n
}

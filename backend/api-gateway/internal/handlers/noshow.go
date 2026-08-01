package handlers

// Employee no-show detection + multi-channel notify (Phase 4/5).
//
//   GET  /api/v1/qa/employee-no-shows?date=YYYY-MM-DD   — staff with no biometric check-in by 09:30
//   POST /api/v1/qa/notify-no-shows  {date?}            — email + WhatsApp today's no-shows (QA action)
//   POST /internal/cron/notify-no-shows                 — same, all tenants, for a scheduled Render cron
//                                                          (gated by the X-Cron-Secret header == CRON_SECRET)
//
// Notifications go to the notification-service (NOTIFICATION_URL) /notify/absentees, which emails and
// WhatsApps each recipient (WhatsApp is a no-op-with-log until provider creds are set).

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

type noShow struct {
	StaffID string `json:"staff_id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
}

// queryNoShows returns the employees who have NO biometric check-in on `date` before the 09:30
// deadline, for one tenant. Uses adminPool (explicit tenant filter).
func queryNoShows(r *http.Request, adminPool *pgxpool.Pool, tenantID, date string) ([]noShow, error) {
	rows, err := adminPool.Query(r.Context(), `
		SELECT e.staff_id, e.full_name, COALESCE(e.email,''), COALESCE(e.phone,'')
		FROM employees e
		WHERE e.tenant_id = $1
		  AND NOT EXISTS (
		      SELECT 1 FROM employee_attendance_logs a
		      WHERE a.tenant_id = e.tenant_id AND a.staff_id = e.staff_id
		        AND a.event_time::date = $2::date
		        AND a.event_time::time <= TIME '09:30')
		ORDER BY e.full_name`, tenantID, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []noShow{}
	for rows.Next() {
		var n noShow
		if rows.Scan(&n.StaffID, &n.Name, &n.Email, &n.Phone) == nil {
			out = append(out, n)
		}
	}
	return out, nil
}

func today() string { return time.Now().Format("2006-01-02") }

// EmployeeNoShows — list today's (or ?date=) no-shows for the caller's tenant.
func EmployeeNoShows(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		date := r.URL.Query().Get("date")
		if date == "" {
			date = today()
		}
		list, err := queryNoShows(r, adminPool, tenantID, date)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"date": date, "no_shows": list})
	}
}

// notifyURL is the notification-service base (email + WhatsApp fan-out).
func notifyURL() string {
	if u := os.Getenv("NOTIFICATION_URL"); u != "" {
		return u
	}
	return "http://notification-service:8080"
}

// postAbsentees sends the no-shows to the notification-service. Best-effort; returns (emailed?, err).
func postAbsentees(r *http.Request, list []noShow, tenantName string) error {
	recips := make([]map[string]string, 0, len(list))
	for _, n := range list {
		if n.Email == "" && n.Phone == "" {
			continue
		}
		recips = append(recips, map[string]string{"name": n.Name, "email": n.Email, "phone": n.Phone})
	}
	if len(recips) == 0 {
		return nil
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"recipients": recips,
		"subject":    "You have not checked in today",
		"message":    "Our records show you had not checked in via the biometric scanner by 09:30 today. If this is an error, please contact HR/QA.",
		"branding":   map[string]string{"name": tenantName, "domain": "qaat"},
	})
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, notifyURL()+"/notify/absentees", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// NotifyNoShows — QA officer triggers the email+WhatsApp fan-out for today's no-shows.
func NotifyNoShows(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		date := today()
		var body struct {
			Date string `json:"date"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Date != "" {
			date = body.Date
		}
		list, err := queryNoShows(r, adminPool, tenantID, date)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		var tenantName string
		_ = adminPool.QueryRow(r.Context(), `SELECT COALESCE(name,'') FROM tenants WHERE tenant_id=$1`, tenantID).Scan(&tenantName)
		if err := postAbsentees(r, list, tenantName); err != nil {
			writeJSON(w, http.StatusBadGateway, errBody("NOTIFY_FAILED", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"date": date, "notified": len(list)})
	}
}

// CronNotifyNoShows — for a scheduled Render cron: notifies no-shows across ALL tenants. Gated by a
// shared secret header so it isn't publicly callable.
func CronNotifyNoShows(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secret := os.Getenv("CRON_SECRET")
		if secret == "" || r.Header.Get("X-Cron-Secret") != secret {
			writeJSON(w, http.StatusUnauthorized, errBody("UNAUTHORIZED", "bad cron secret"))
			return
		}
		date := today()
		rows, err := adminPool.Query(r.Context(), `SELECT tenant_id::text, COALESCE(name,'') FROM tenants`)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		type t struct{ id, name string }
		var tenants []t
		for rows.Next() {
			var x t
			if rows.Scan(&x.id, &x.name) == nil {
				tenants = append(tenants, x)
			}
		}
		rows.Close()

		total := 0
		for _, tn := range tenants {
			list, qerr := queryNoShows(r, adminPool, tn.id, date)
			if qerr != nil {
				continue
			}
			_ = postAbsentees(r, list, tn.name)
			total += len(list)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"date": date, "tenants": len(tenants), "notified": total})
	}
}

package handlers

// Cross-role in-app notifications (workstream D). A LECTURER notifies the students of his
// unit(s) or the coordinator(s) of those units; a COORDINATOR notifies his cohort's students
// or the lecturer(s) of his course units. Recipients are materialised at send time so a
// reader's inbox (student / coordinator / lecturer) is a simple, fast lookup.
//
//   POST /api/v1/app-notifications                       (LECTURER, COORDINATOR)
//   GET  /api/v1/app-notifications                       (any signed-in app user)
//   GET  /api/v1/app-notifications/unread-count          (any signed-in app user)
//   POST /api/v1/app-notifications/{id}/read             (any signed-in app user)

import (
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// resolveRecipients returns the distinct recipient user_ids for a send, based on the sender's
// role + the requested audience + an optional unit scope.
func resolveRecipients(pool *pgxpool.Pool, r *http.Request, tenantID, senderID, senderRole, audience, unitID string) ([]string, error) {
	var sql string
	args := []interface{}{tenantID}

	switch senderRole {
	case middleware.RoleLecturer:
		lecturerID, ok := resolveLecturerID(pool, r, tenantID, senderID)
		if !ok {
			return nil, nil
		}
		args = append(args, lecturerID) // $2
		unitClause := ""
		if unitID != "" {
			args = append(args, unitID)
			unitClause = " AND cu.unit_id = $3"
		}
		switch audience {
		case "STUDENTS":
			sql = `SELECT DISTINCT u.user_id::text
				FROM lecturer_assignments la
				JOIN course_units cu     ON cu.unit_id = la.unit_id AND cu.tenant_id = la.tenant_id
				JOIN course_offerings o  ON o.course_id = cu.course_id AND o.tenant_id = cu.tenant_id
				JOIN students_extended s ON s.offering_id = o.offering_id AND s.tenant_id = o.tenant_id AND s.enrollment_status='ACTIVE'
				JOIN users u ON lower(u.email) = lower(s.email) AND u.tenant_id = s.tenant_id
				WHERE la.tenant_id = $1 AND la.lecturer_id = $2::uuid` + unitClause
		case "COORDINATOR":
			sql = `SELECT DISTINCT o.coordinator_id
				FROM lecturer_assignments la
				JOIN course_units cu    ON cu.unit_id = la.unit_id AND cu.tenant_id = la.tenant_id
				JOIN course_offerings o ON o.course_id = cu.course_id AND o.tenant_id = cu.tenant_id
				WHERE la.tenant_id = $1 AND la.lecturer_id = $2::uuid AND COALESCE(o.coordinator_id,'') <> ''` + unitClause
		default:
			return nil, nil
		}

	case middleware.RoleCoordinator:
		args = append(args, senderID) // $2 = coordinator user_id
		unitClause := ""
		if unitID != "" {
			args = append(args, unitID)
			unitClause = " AND cu.unit_id = $3"
		}
		switch audience {
		case "STUDENTS":
			sql = `SELECT DISTINCT u.user_id::text
				FROM course_offerings o
				JOIN students_extended s ON s.offering_id = o.offering_id AND s.tenant_id = o.tenant_id AND s.enrollment_status='ACTIVE'
				JOIN users u ON lower(u.email) = lower(s.email) AND u.tenant_id = s.tenant_id
				WHERE o.tenant_id = $1 AND o.coordinator_id = $2`
		case "LECTURERS":
			sql = `SELECT DISTINCT l.user_id::text
				FROM course_offerings o
				JOIN course_units cu ON cu.course_id = o.course_id AND cu.tenant_id = o.tenant_id
				JOIN lecturer_assignments la ON la.unit_id = cu.unit_id AND la.tenant_id = o.tenant_id
				JOIN lecturers l ON l.lecturer_id = la.lecturer_id AND l.tenant_id = la.tenant_id
				WHERE o.tenant_id = $1 AND o.coordinator_id = $2 AND l.user_id IS NOT NULL` + unitClause
		default:
			return nil, nil
		}
	default:
		return nil, nil
	}

	rows, err := pool.Query(r.Context(), sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil && id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// SendAppNotification — POST /api/v1/app-notifications
func SendAppNotification(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		senderID := middleware.GetUserID(r.Context())
		role := middleware.GetRole(r.Context())
		if role != middleware.RoleLecturer && role != middleware.RoleCoordinator {
			writeJSON(w, http.StatusForbidden, errBody("FORBIDDEN", "only a lecturer or coordinator can send notifications"))
			return
		}

		var req struct {
			Audience string `json:"audience"`
			UnitID   string `json:"unit_id"`
			Subject  string `json:"subject"`
			Body     string `json:"body"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		req.Audience = strings.ToUpper(strings.TrimSpace(req.Audience))
		req.UnitID = strings.TrimSpace(req.UnitID)
		req.Subject = strings.TrimSpace(req.Subject)
		if req.Subject == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "subject is required"))
			return
		}
		// Validate audience against the sender's role.
		valid := map[string]map[string]bool{
			middleware.RoleLecturer:    {"STUDENTS": true, "COORDINATOR": true},
			middleware.RoleCoordinator: {"STUDENTS": true, "LECTURERS": true},
		}
		if !valid[role][req.Audience] {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "invalid audience for your role"))
			return
		}

		recipients, err := resolveRecipients(pool, r, tenantID, senderID, role, req.Audience, req.UnitID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		// Don't notify yourself.
		filtered := recipients[:0]
		for _, id := range recipients {
			if id != senderID {
				filtered = append(filtered, id)
			}
		}
		recipients = filtered
		if len(recipients) == 0 {
			writeJSON(w, http.StatusOK, map[string]interface{}{"status": "SENT", "recipients": 0})
			return
		}

		var senderName string
		_ = pool.QueryRow(r.Context(), `SELECT full_name FROM users WHERE user_id = $1 AND tenant_id = $2`, senderID, tenantID).Scan(&senderName)
		if senderName == "" {
			senderName = role
		}

		tx, err := pool.Begin(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "tx"))
			return
		}
		defer tx.Rollback(r.Context()) //nolint:errcheck

		var nid string
		err = tx.QueryRow(r.Context(), `
			INSERT INTO app_notifications (tenant_id, sender_id, sender_name, sender_role, audience, unit_id, subject, body)
			VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8) RETURNING notification_id::text`,
			tenantID, senderID, senderName, role, req.Audience, req.UnitID, req.Subject, req.Body).Scan(&nid)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		for _, uid := range recipients {
			_, err = tx.Exec(r.Context(), `
				INSERT INTO notification_recipients (notification_id, tenant_id, recipient_user_id)
				VALUES ($1,$2,$3::uuid) ON CONFLICT DO NOTHING`, nid, tenantID, uid)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
				return
			}
		}
		if err := tx.Commit(r.Context()); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "commit"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "SENT", "notification_id": nid, "recipients": len(recipients)})
	}
}

// ListAppNotifications — GET /api/v1/app-notifications (the caller's inbox).
func ListAppNotifications(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		rows, err := pool.Query(r.Context(), `
			SELECT n.notification_id::text, n.sender_name, n.sender_role, COALESCE(n.unit_id,''),
			       n.subject, n.body, n.created_at, (nr.read_at IS NOT NULL)
			FROM notification_recipients nr
			JOIN app_notifications n ON n.notification_id = nr.notification_id
			WHERE nr.tenant_id = $1 AND nr.recipient_user_id = $2::uuid
			ORDER BY n.created_at DESC LIMIT 300`, tenantID, userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()
		type notif struct {
			NotificationID string `json:"notification_id"`
			SenderName     string `json:"sender_name"`
			SenderRole     string `json:"sender_role"`
			UnitID         string `json:"unit_id"`
			Subject        string `json:"subject"`
			Body           string `json:"body"`
			CreatedAt      string `json:"created_at"`
			Read           bool   `json:"read"`
		}
		out := []notif{}
		for rows.Next() {
			var n notif
			var created time.Time
			if rows.Scan(&n.NotificationID, &n.SenderName, &n.SenderRole, &n.UnitID, &n.Subject, &n.Body, &created, &n.Read) != nil {
				continue
			}
			n.CreatedAt = created.Format(time.RFC3339)
			out = append(out, n)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// UnreadAppNotificationCount — GET /api/v1/app-notifications/unread-count
func UnreadAppNotificationCount(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		var n int
		_ = pool.QueryRow(r.Context(),
			`SELECT count(*) FROM notification_recipients WHERE tenant_id=$1 AND recipient_user_id=$2::uuid AND read_at IS NULL`,
			tenantID, userID).Scan(&n)
		writeJSON(w, http.StatusOK, map[string]int{"unread": n})
	}
}

// MarkAppNotificationRead — POST /api/v1/app-notifications/{id}/read
func MarkAppNotificationRead(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		id := extractPathID(r.URL.Path, "/api/v1/app-notifications/", "/read")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "missing id"))
			return
		}
		_, err := pool.Exec(r.Context(),
			`UPDATE notification_recipients SET read_at = now()
			 WHERE notification_id = $1::uuid AND recipient_user_id = $2::uuid AND tenant_id = $3 AND read_at IS NULL`,
			id, userID, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "READ"})
	}
}

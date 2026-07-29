package handlers

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// DQA ⇄ QA-officer in-app messaging. The DQA director shares reports/notifications to
// QA officers (all / by department / by college-school); QA officers message the DQA back.
// Optional inline file attachment. adminPool + explicit tenant scoping on every query.

const maxAttachmentBytes = 8 << 20 // 8 MiB

// SendQAMessage — POST /api/v1/messages
func SendQAMessage(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		senderID := middleware.GetUserID(r.Context())
		role := middleware.GetRole(r.Context())

		var req struct {
			Audience       string `json:"audience"`
			AudienceValue  string `json:"audience_value"`
			Subject        string `json:"subject"`
			Body           string `json:"body"`
			AttachmentName string `json:"attachment_name"`
			AttachmentMime string `json:"attachment_mime"`
			AttachmentB64  string `json:"attachment_b64"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		req.Audience = strings.TrimSpace(req.Audience)
		req.AudienceValue = strings.TrimSpace(req.AudienceValue)
		req.Subject = strings.TrimSpace(req.Subject)
		if req.Subject == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "subject is required"))
			return
		}

		// RBAC on audience: the DQA broadcasts down to QA officers; QA officers reply up to the DQA.
		switch role {
		case middleware.RoleDQADirector:
			if req.Audience != "ALL_QA" && req.Audience != "DEPARTMENT" && req.Audience != "SCHOOL" {
				writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "audience must be ALL_QA, DEPARTMENT or SCHOOL"))
				return
			}
			if (req.Audience == "DEPARTMENT" || req.Audience == "SCHOOL") && req.AudienceValue == "" {
				writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "pick a department or school"))
				return
			}
			if req.Audience == "ALL_QA" {
				req.AudienceValue = ""
			}
		case middleware.RoleQAOfficer:
			req.Audience = "DQA" // QA officers can only message the DQA director
			req.AudienceValue = ""
		default:
			writeJSON(w, http.StatusForbidden, errBody("FORBIDDEN", "only the DQA director and QA officers can send messages"))
			return
		}

		// Optional attachment.
		var attData []byte
		var attName, attMime *string
		if req.AttachmentB64 != "" {
			b, err := base64.StdEncoding.DecodeString(req.AttachmentB64)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "attachment is not valid base64"))
				return
			}
			if len(b) > maxAttachmentBytes {
				writeJSON(w, http.StatusRequestEntityTooLarge, errBody("ATTACHMENT_TOO_LARGE", "attachment exceeds 8 MB"))
				return
			}
			attData = b
			n := req.AttachmentName
			if n == "" {
				n = "attachment"
			}
			m := req.AttachmentMime
			if m == "" {
				m = "application/octet-stream"
			}
			attName, attMime = &n, &m
		}

		// Sender display name.
		var senderName string
		_ = pool.QueryRow(r.Context(),
			`SELECT full_name FROM users WHERE user_id = $1 AND tenant_id = $2`, senderID, tenantID).Scan(&senderName)
		if senderName == "" {
			senderName = role
		}

		var id string
		err := pool.QueryRow(r.Context(), `
			INSERT INTO qa_messages
			  (tenant_id, sender_id, sender_name, sender_role, audience, audience_value,
			   subject, body, attachment_name, attachment_mime, attachment_data)
			VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9,$10,$11)
			RETURNING message_id::text`,
			tenantID, senderID, senderName, role, req.Audience, req.AudienceValue,
			req.Subject, req.Body, attName, attMime, attData,
		).Scan(&id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message_id": id, "status": "SENT"})
	}
}

type qaMessageOut struct {
	MessageID     string  `json:"message_id"`
	SenderName    string  `json:"sender_name"`
	SenderRole    string  `json:"sender_role"`
	Audience      string  `json:"audience"`
	AudienceValue string  `json:"audience_value"`
	Subject       string  `json:"subject"`
	Body          string  `json:"body"`
	HasAttachment bool    `json:"has_attachment"`
	AttachmentName string `json:"attachment_name"`
	CreatedAt     string  `json:"created_at"`
	Read          bool    `json:"read"`
}

// ListQAMessages — GET /api/v1/messages?box=inbox|sent
func ListQAMessages(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		role := middleware.GetRole(r.Context())
		box := r.URL.Query().Get("box")
		if box == "" {
			box = "inbox"
		}

		// The caller's own department/school (drives which DEPARTMENT/SCHOOL messages reach a QA officer).
		var dept, school string
		_ = pool.QueryRow(r.Context(),
			`SELECT COALESCE(department,''), COALESCE(school,'') FROM users WHERE user_id = $1 AND tenant_id = $2`,
			userID, tenantID).Scan(&dept, &school)

		// The read flag is a LEFT JOIN on this caller's read rows.
		base := `
			SELECT m.message_id::text, m.sender_name, m.sender_role, m.audience,
			       COALESCE(m.audience_value,''), m.subject, m.body,
			       (m.attachment_data IS NOT NULL), COALESCE(m.attachment_name,''), m.created_at,
			       (rd.user_id IS NOT NULL) AS is_read
			FROM qa_messages m
			LEFT JOIN qa_message_reads rd ON rd.message_id = m.message_id AND rd.user_id = $2
			WHERE m.tenant_id = $1 `

		var cond string
		args := []interface{}{tenantID, userID}
		switch {
		case box == "sent":
			cond = "AND m.sender_id = $2 "
		case role == middleware.RoleDQADirector:
			// DQA inbox = replies from QA officers.
			cond = "AND m.audience = 'DQA' "
		case role == middleware.RoleQAOfficer:
			// QA inbox = broadcasts that target them: all QA, their department, or their school.
			cond = `AND ( m.audience = 'ALL_QA'
			             OR (m.audience = 'DEPARTMENT' AND m.audience_value = $3)
			             OR (m.audience = 'SCHOOL'     AND m.audience_value = $4) ) `
			args = append(args, dept, school)
		default:
			writeJSON(w, http.StatusForbidden, errBody("FORBIDDEN", "not a QA officer or DQA director"))
			return
		}

		rows, err := pool.Query(r.Context(), base+cond+"ORDER BY m.created_at DESC LIMIT 500", args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		out := []qaMessageOut{}
		for rows.Next() {
			var m qaMessageOut
			var created time.Time
			if err := rows.Scan(&m.MessageID, &m.SenderName, &m.SenderRole, &m.Audience, &m.AudienceValue,
				&m.Subject, &m.Body, &m.HasAttachment, &m.AttachmentName, &created, &m.Read); err != nil {
				continue
			}
			m.CreatedAt = created.Format(time.RFC3339)
			out = append(out, m)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// UnreadQAMessageCount — GET /api/v1/messages/unread-count (badge for the caller's inbox).
func UnreadQAMessageCount(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		role := middleware.GetRole(r.Context())

		var dept, school string
		_ = pool.QueryRow(r.Context(),
			`SELECT COALESCE(department,''), COALESCE(school,'') FROM users WHERE user_id = $1 AND tenant_id = $2`,
			userID, tenantID).Scan(&dept, &school)

		q := `SELECT count(*) FROM qa_messages m
		      LEFT JOIN qa_message_reads rd ON rd.message_id = m.message_id AND rd.user_id = $2
		      WHERE m.tenant_id = $1 AND rd.user_id IS NULL AND m.sender_id <> $2 AND `
		args := []interface{}{tenantID, userID}
		switch role {
		case middleware.RoleDQADirector:
			q += "m.audience = 'DQA'"
		case middleware.RoleQAOfficer:
			q += "(m.audience='ALL_QA' OR (m.audience='DEPARTMENT' AND m.audience_value=$3) OR (m.audience='SCHOOL' AND m.audience_value=$4))"
			args = append(args, dept, school)
		default:
			writeJSON(w, http.StatusOK, map[string]int{"unread": 0})
			return
		}
		var n int
		_ = pool.QueryRow(r.Context(), q, args...).Scan(&n)
		writeJSON(w, http.StatusOK, map[string]int{"unread": n})
	}
}

// MarkQAMessageRead — POST /api/v1/messages/{id}/read
func MarkQAMessageRead(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		id := extractPathID(r.URL.Path, "/api/v1/messages/", "/read")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "missing message id"))
			return
		}
		// Only record a read if the message exists for this tenant (scoping).
		_, err := pool.Exec(r.Context(), `
			INSERT INTO qa_message_reads (message_id, tenant_id, user_id)
			SELECT $1::uuid, $2, $3
			WHERE EXISTS (SELECT 1 FROM qa_messages WHERE message_id = $1::uuid AND tenant_id = $2)
			ON CONFLICT (message_id, user_id) DO NOTHING`,
			id, tenantID, userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "READ"})
	}
}

// QAAudiences — GET /api/v1/messages/audiences → the distinct departments and
// college/schools that QA officers belong to, so the DQA composer targets real values.
func QAAudiences(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		depts := []string{}
		schools := []string{}
		rows, err := pool.Query(r.Context(),
			`SELECT DISTINCT COALESCE(department,''), COALESCE(school,'')
			 FROM users WHERE tenant_id = $1 AND role = 'QA_OFFICER'`, tenantID)
		if err == nil {
			defer rows.Close()
			dset, sset := map[string]bool{}, map[string]bool{}
			for rows.Next() {
				var d, s string
				if rows.Scan(&d, &s) == nil {
					if d != "" && !dset[d] {
						dset[d] = true
						depts = append(depts, d)
					}
					if s != "" && !sset[s] {
						sset[s] = true
						schools = append(schools, s)
					}
				}
			}
		}
		writeJSON(w, http.StatusOK, map[string][]string{"departments": depts, "schools": schools})
	}
}

// QAMessageAttachment — GET /api/v1/messages/{id}/attachment (streams the stored file).
func QAMessageAttachment(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		id := extractPathID(r.URL.Path, "/api/v1/messages/", "/attachment")
		if id == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		var name, mime string
		var data []byte
		err := pool.QueryRow(r.Context(),
			`SELECT COALESCE(attachment_name,'file'), COALESCE(attachment_mime,'application/octet-stream'), attachment_data
			 FROM qa_messages WHERE message_id = $1::uuid AND tenant_id = $2`, id, tenantID).Scan(&name, &mime, &data)
		if err != nil || data == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", mime)
		w.Header().Set("Content-Disposition", "attachment; filename=\""+strings.ReplaceAll(name, "\"", "")+"\"")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

package handlers

// Employee (staff) attendance — imported from the physical check-in tablet, and
// the report admins read. Each tablet punch is staff_id + timestamp + IN/OUT
// (+ optional title/name/comment). The report pairs each day's IN and OUT and
// auto-writes a Comment cell: the latest date/time plus the running count of days
// the employee actually worked (a "worked" day = one with a successful in AND out).

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// reportLoc groups punches into local calendar days (container TZ is Africa/Kampala).
var reportLoc = func() *time.Location {
	if l, err := time.LoadLocation("Africa/Kampala"); err == nil {
		return l
	}
	return time.UTC
}()

// normalizeEventType maps free-text in/out labels to IN | OUT | PUNCH.
func normalizeEventType(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "in", "i", "check-in", "checkin", "check in", "clock-in", "clockin", "clock in", "entry":
		return "IN"
	case "out", "o", "check-out", "checkout", "check out", "clock-out", "clockout", "clock out", "exit":
		return "OUT"
	default:
		return "PUNCH"
	}
}

var punchTimeLayouts = []string{
	time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04",
	"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02",
	"02/01/2006 15:04:05", "02/01/2006 15:04", "02/01/2006",
	"01/02/2006 15:04:05", "01/02/2006 15:04", "01/02/2006",
	"2006/01/02 15:04:05", "2006/01/02 15:04", "2006/01/02",
	"01-02-2006 15:04", "02-01-2006 15:04",
	"3:04 PM", "15:04",
}

// parsePunchTime accepts a single datetime string, or a date + separate time.
func parsePunchTime(dt, dateOnly, timeOnly string) (time.Time, bool) {
	try := func(s string) (time.Time, bool) {
		s = strings.TrimSpace(s)
		if s == "" {
			return time.Time{}, false
		}
		for _, layout := range punchTimeLayouts {
			if t, err := time.ParseInLocation(layout, s, reportLoc); err == nil {
				return t, true
			}
		}
		return time.Time{}, false
	}
	if t, ok := try(dt); ok {
		return t, true
	}
	// date + time columns combined
	d, t := strings.TrimSpace(dateOnly), strings.TrimSpace(timeOnly)
	if d != "" {
		if combined, ok := try(strings.TrimSpace(d + " " + t)); ok {
			return combined, true
		}
		if only, ok := try(d); ok {
			return only, true
		}
	}
	return time.Time{}, false
}

// POST /api/v1/admin/tenants/{tenant_id}/employee-attendance/import  (field "punches")
func ImportEmployeePunches(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "expected multipart/form-data"))
			return
		}
		file, _, err := r.FormFile("punches")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "field 'punches' not found"))
			return
		}
		defer file.Close()

		rows, idx, err := readTabular(file)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("CSV_PARSE_ERROR", err.Error()))
			return
		}
		if _, ok := idx["staff_id"]; !ok {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("CSV_PARSE_ERROR", "missing required column: staff_id"))
			return
		}

		// pick the first present datetime-ish column name.
		dtCol := ""
		for _, c := range []string{"datetime", "date_time", "timestamp", "event_time", "time_stamp", "punch_time"} {
			if _, ok := idx[c]; ok {
				dtCol = c
				break
			}
		}

		res := &importResult{Errors: []string{}}
		for ln := 1; ln < len(rows); ln++ {
			row := rows[ln]
			staffID := cell(row, idx, "staff_id")
			if staffID == "" {
				res.Skipped++
				continue
			}
			evtTime, ok := parsePunchTime(cell(row, idx, dtCol), cell(row, idx, "date"), cell(row, idx, "time"))
			if !ok {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d: could not read a date/time for %s", ln, staffID))
				continue
			}
			evtType := normalizeEventType(firstNonEmpty(cell(row, idx, "in/out"), cell(row, idx, "in_out"),
				cell(row, idx, "type"), cell(row, idx, "direction"), cell(row, idx, "status")))
			comment := firstNonEmpty(cell(row, idx, "comment"), cell(row, idx, "note"), cell(row, idx, "remarks"))

			// Auto-register a stub employee if the badge isn't in the registry yet, so
			// the punch always resolves to a named person on the report.
			name := firstNonEmpty(cell(row, idx, "full_name"), cell(row, idx, "name"))
			title := cell(row, idx, "title")
			if name == "" {
				name = staffID
			}
			_, _ = adminPool.Exec(r.Context(), `
				INSERT INTO employees (tenant_id, staff_id, title, full_name)
				VALUES ($1,$2,NULLIF($3,''),$4)
				ON CONFLICT (tenant_id, staff_id) DO UPDATE SET
				    title = COALESCE(employees.title, NULLIF(EXCLUDED.title,''))`,
				tenantID, staffID, title, name)

			ct, e := adminPool.Exec(r.Context(), `
				INSERT INTO employee_attendance_logs (tenant_id, staff_id, event_time, event_type, source, comment)
				VALUES ($1,$2,$3,$4,'TABLET',NULLIF($5,''))
				ON CONFLICT (tenant_id, staff_id, event_time, event_type) DO NOTHING`,
				tenantID, staffID, evtTime.UTC(), evtType, comment)
			if e != nil {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d: %s", ln, e.Error()))
				continue
			}
			if ct.RowsAffected() == 0 {
				res.Updated++ // duplicate punch (idempotent re-import)
			} else {
				res.Inserted++
			}
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

type punch struct {
	t   time.Time
	typ string
}

// GET /api/v1/admin/tenants/{tenant_id}/employee-attendance?from=YYYY-MM-DD&to=YYYY-MM-DD&department=
func EmployeeAttendanceReport(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		from, to := dateRange(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
		dept := strings.TrimSpace(r.URL.Query().Get("department"))

		// Registry (so every employee shows, even with zero punches).
		empArgs := []interface{}{tenantID}
		empWhere := "tenant_id = $1"
		if dept != "" {
			empArgs = append(empArgs, dept)
			empWhere += fmt.Sprintf(" AND department = $%d", len(empArgs))
		}
		empRows, err := adminPool.Query(r.Context(), `
			SELECT staff_id, COALESCE(title,''), full_name, COALESCE(phone,''), COALESCE(email,'')
			FROM employees WHERE `+empWhere+` ORDER BY full_name`, empArgs...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		type meta struct{ title, name, contact string }
		order := []string{}
		info := map[string]meta{}
		for empRows.Next() {
			var sid, title, name, phone, email string
			if err := empRows.Scan(&sid, &title, &name, &phone, &email); err != nil {
				continue
			}
			info[sid] = meta{title, name, firstNonEmpty(phone, email)}
			order = append(order, sid)
		}
		empRows.Close()

		// Punches in range for this tenant.
		punchRows, err := adminPool.Query(r.Context(), `
			SELECT staff_id, event_time, event_type
			FROM employee_attendance_logs
			WHERE tenant_id = $1 AND event_time >= $2 AND event_time < $3
			ORDER BY staff_id, event_time`, tenantID, from, to)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		byStaff := map[string][]punch{}
		for punchRows.Next() {
			var sid string
			var t time.Time
			var typ string
			if err := punchRows.Scan(&sid, &t, &typ); err != nil {
				continue
			}
			byStaff[sid] = append(byStaff[sid], punch{t.In(reportLoc), typ})
			if _, seen := info[sid]; !seen {
				info[sid] = meta{"", sid, ""}
				order = append(order, sid)
			}
		}
		punchRows.Close()

		// QAAT only displays staff_id, name, contact and the auto-comment — the rest
		// (department, job title, day counts) live inside the comment or not at all.
		type reportRow struct {
			StaffID  string `json:"staff_id"`
			Title    string `json:"title"`
			FullName string `json:"full_name"`
			Contact  string `json:"contact"`
			Comment  string `json:"comment"`
		}
		out := make([]reportRow, 0, len(order))
		fromLabel := from.In(reportLoc).Format("2006-01-02")
		toLabel := to.In(reportLoc).Add(-time.Second).Format("2006-01-02")
		for _, sid := range order {
			m := info[sid]
			summ := summarizePunches(byStaff[sid])
			out = append(out, reportRow{
				StaffID:  sid,
				Title:    m.title,
				FullName: m.name,
				Contact:  m.contact,
				Comment:  buildAttendanceComment(m.title, m.name, summ, fromLabel, toLabel),
			})
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"from": fromLabel, "to": toLabel, "rows": out,
		})
	}
}

type punchSummary struct {
	daysPresent int
	daysWorked  int
	first, last time.Time
	latestDay   string
	latestIn    string
	latestOut   string
}

// summarizePunches pairs each calendar day's punches: a "worked" day has both an
// IN and an OUT (or, when the tablet emits unlabelled punches, at least two).
func summarizePunches(ps []punch) punchSummary {
	var s punchSummary
	if len(ps) == 0 {
		return s
	}
	type dayAgg struct {
		hasIn, hasOut bool
		count         int
		first, last   time.Time
	}
	days := map[string]*dayAgg{}
	for _, p := range ps {
		key := p.t.Format("2006-01-02")
		d := days[key]
		if d == nil {
			d = &dayAgg{first: p.t, last: p.t}
			days[key] = d
		}
		if p.t.Before(d.first) {
			d.first = p.t
		}
		if p.t.After(d.last) {
			d.last = p.t
		}
		d.count++
		switch p.typ {
		case "IN":
			d.hasIn = true
		case "OUT":
			d.hasOut = true
		}
		if s.first.IsZero() || p.t.Before(s.first) {
			s.first = p.t
		}
		if p.t.After(s.last) {
			s.last = p.t
		}
	}
	keys := make([]string, 0, len(days))
	for k := range days {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	s.daysPresent = len(keys)
	for _, k := range keys {
		d := days[k]
		if (d.hasIn && d.hasOut) || d.count >= 2 {
			s.daysWorked++
		}
	}
	// latest day detail for the comment
	last := days[keys[len(keys)-1]]
	s.latestDay = keys[len(keys)-1]
	s.latestIn = last.first.Format("15:04")
	s.latestOut = last.last.Format("15:04")
	return s
}

// buildAttendanceComment is the auto-generated Comment cell for the report.
func buildAttendanceComment(title, name string, s punchSummary, from, to string) string {
	who := strings.TrimSpace(title + " " + name)
	if s.daysPresent == 0 {
		return fmt.Sprintf("%s — no tablet check-ins between %s and %s.", who, from, to)
	}
	return fmt.Sprintf(
		"%s worked %d day(s) with a successful check-in & out (of %d day(s) present) between %s and %s. Latest: in %s & out %s on %s.",
		who, s.daysWorked, s.daysPresent, from, to, s.latestIn, s.latestOut, s.latestDay)
}

// dateRange parses optional from/to (YYYY-MM-DD) into a [from, to) UTC window.
// Defaults: the last 30 days through the end of today.
func dateRange(fromStr, toStr string) (time.Time, time.Time) {
	now := time.Now().In(reportLoc)
	parseDay := func(s string) (time.Time, bool) {
		if t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(s), reportLoc); err == nil {
			return t, true
		}
		return time.Time{}, false
	}
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, reportLoc).AddDate(0, 0, 1)
	if t, ok := parseDay(toStr); ok {
		to = t.AddDate(0, 0, 1) // inclusive of the 'to' day
	}
	from := to.AddDate(0, 0, -30)
	if f, ok := parseDay(fromStr); ok {
		from = f
	}
	return from.UTC(), to.UTC()
}

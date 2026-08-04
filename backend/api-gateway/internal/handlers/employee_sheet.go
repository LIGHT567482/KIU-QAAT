package handlers

// The biometric terminal's daily attendance sheet.
//
//   POST /api/v1/admin/tenants/{tenant_id}/employee-attendance/sheet — upload
//   GET  /api/v1/dashboard/employee-days                             — filtered read
//   GET  /api/v1/dashboard/employee-days/export.{xlsx,csv,pdf}       — the same, exported
//
// The terminal exports one row per employee per day with 29 columns, and that sheet is what HR
// already works from: it encodes the shift ("GENERAL" = 08:00–17:00), the overtime arithmetic and
// the holiday calendar the institution has already agreed. Recomputing those here would mean
// reimplementing a payroll policy and then disagreeing with it, so the sheet is stored as it
// arrives and everything else sits on top.
//
// Two things ARE derived on the way in: whether clock-in was after on-duty, and whether clock-out
// was before off-duty. The terminal's own Late/Early columns are frequently blank even when the
// times say otherwise, and the notification jobs need a boolean they can filter on every sweep
// rather than TEXT times to parse per row.

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// sheetAliases maps the export's human headers onto our column names. The terminal's spelling
// varies between firmware versions ("AC-No." / "AC No" / "acno"), so matching is on a normalised
// form rather than the literal string.
var sheetAliases = map[string]string{
	"empno": "emp_no", "empno.": "emp_no", "employeeno": "emp_no", "employeenumber": "emp_no",
	"acno": "ac_no", "acno.": "ac_no", "accessno": "ac_no", "cardno": "ac_no",
	"no": "seq_no", "no.": "seq_no",
	"name": "full_name", "employeename": "full_name", "fullname": "full_name",
	"autoassign": "auto_assign",
	"date":       "work_date",
	"timetable":  "timetable",
	"onduty":     "on_duty", "offduty": "off_duty",
	"clockin": "clock_in", "clockout": "clock_out",
	"normal": "normal", "realtime": "real_time",
	"late": "late", "early": "early", "absent": "absent",
	"ottime": "ot_time", "worktime": "work_time", "exception": "exception",
	"mustcin": "must_cin", "mustc/in": "must_cin", "mustcheckin": "must_cin",
	"mustcout": "must_cout", "mustc/out": "must_cout", "mustcheckout": "must_cout",
	"department": "department",
	"ndays":      "ndays", "weekend": "weekend", "holiday": "holiday",
	"atttime": "att_time", "att_time": "att_time",
	"ndaysot": "ndays_ot", "ndays_ot": "ndays_ot",
	"weekendot": "weekend_ot", "weekend_ot": "weekend_ot",
	"holidayot": "holiday_ot", "holiday_ot": "holiday_ot",
}

// normaliseSheetHeader strips everything that varies between exports: case, spaces, underscores
// and the trailing period the terminal puts on abbreviations.
func normaliseSheetHeader(h string) string {
	s := strings.ToLower(strings.TrimSpace(h))
	s = strings.NewReplacer(" ", "", "_", "", "-", "", " ", "").Replace(s)
	if mapped, ok := sheetAliases[s]; ok {
		return mapped
	}
	return s
}

// looksLikeDaySheet decides whether an upload is this 29-column export or the older punch file.
// Both arrive at the same screen, so guessing wrong silently imports the wrong shape.
func looksLikeDaySheet(header []string) bool {
	seen := map[string]bool{}
	for _, h := range header {
		seen[normaliseSheetHeader(h)] = true
	}
	// On duty + Off duty together are the signature: the punch export has neither.
	return seen["ac_no"] && seen["on_duty"] && seen["off_duty"]
}

// parseSheetDay reads the export's DD/MM/YYYY. Day-first is not a guess: the sample runs
// 27/07/2026 … 31/07/2026, which is only a valid month-first date for none of them — but a file
// covering the first twelve days of a month would be ambiguous, so the order is fixed rather than
// sniffed. Falls back to the ISO and slash-ISO spellings some exports use.
func parseSheetDay(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"02/01/2006", "02-01-2006", "2006-01-02", "2006/01/02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	// Excel hands a formatted date cell over as a serial number.
	if f, err := strconv.ParseFloat(s, 64); err == nil && f > 20000 && f < 80000 {
		return time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(f)), true
	}
	return time.Time{}, false
}

// sheetBool reads the export's True / blank convention. Anything that is not affirmative is false,
// including the empty string, which is how the sheet spells "no".
func sheetBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "y", "1", "t":
		return true
	}
	return false
}

// sheetNum reads a numeric cell, returning nil for blank so the column stays NULL rather than
// becoming a misleading zero — "no overtime recorded" and "zero overtime" are different claims.
func sheetNum(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}

// hhmmMinutes converts "08:00" to minutes since midnight, for the late/early comparison.
func hhmmMinutes(s string) (int, bool) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return 0, false
	}
	h, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	m, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return 0, false
	}
	return h*60 + m, true
}

// ImportEmployeeSheet ingests the terminal's daily export.
func ImportEmployeeSheet(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "expected multipart/form-data"))
			return
		}
		// Accept either field name: the punch importer uses "punches" and the roster one "roster",
		// and an operator uploading the wrong-named field would otherwise get a bare 400.
		file, hdr, err := r.FormFile("sheet")
		if err != nil {
			if file, hdr, err = r.FormFile("punches"); err != nil {
				writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "attach the exported sheet as 'sheet'"))
				return
			}
		}
		defer file.Close()

		rows, _, err := readTabular(file)
		if err != nil || len(rows) < 2 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_FILE", "could not read the file, or it has no rows"))
			return
		}
		header := rows[0]
		if !looksLikeDaySheet(header) {
			writeJSON(w, http.StatusBadRequest, errBody("WRONG_FORMAT",
				"This does not look like the daily attendance export — it needs at least AC-No., On duty and Off duty columns."))
			return
		}

		col := map[string]int{}
		for i, h := range header {
			col[normaliseSheetHeader(h)] = i
		}
		get := func(row []string, name string) string {
			if i, ok := col[name]; ok && i < len(row) {
				return strings.TrimSpace(row[i])
			}
			return ""
		}

		fileName := ""
		if hdr != nil {
			fileName = sanitizeFilename(hdr.Filename)
		}

		var inserted, skipped int
		var problems []string
		for n, row := range rows[1:] {
			acNo := get(row, "ac_no")
			day, dateOK := parseSheetDay(get(row, "work_date"))
			if acNo == "" || !dateOK {
				skipped++
				if len(problems) < 5 {
					problems = append(problems, fmt.Sprintf("row %d: needs an AC-No. and a readable date", n+2))
				}
				continue
			}

			// Derived flags. Only meaningful when both sides of the comparison exist: an employee
			// who never clocked in is ABSENT, not "late", and calling them late would be a
			// different and wrong accusation.
			clockIn, clockOut := get(row, "clock_in"), get(row, "clock_out")
			onDuty, offDuty := get(row, "on_duty"), get(row, "off_duty")
			late := false
			if ci, ok1 := hhmmMinutes(clockIn); ok1 {
				if od, ok2 := hhmmMinutes(onDuty); ok2 {
					late = ci > od
				}
			}
			early := false
			if co, ok1 := hhmmMinutes(clockOut); ok1 {
				if od, ok2 := hhmmMinutes(offDuty); ok2 {
					early = co < od
				}
			}

			_, execErr := adminPool.Exec(r.Context(), `
				INSERT INTO employee_attendance_days
				  (tenant_id, emp_no, ac_no, seq_no, full_name, auto_assign, department,
				   work_date, timetable, on_duty, off_duty, clock_in, clock_out,
				   normal, real_time, late, early, absent, ot_time, work_time, exception,
				   must_cin, must_cout, ndays, weekend, holiday, att_time,
				   ndays_ot, weekend_ot, holiday_ot,
				   checked_in_late, checked_out_early, source_file)
				VALUES ($1,NULLIF($2,''),$3,NULLIF($4,''),$5,NULLIF($6,''),NULLIF($7,''),
				        $8::date,NULLIF($9,''),NULLIF($10,''),NULLIF($11,''),NULLIF($12,''),NULLIF($13,''),
				        $14,$15,NULLIF($16,''),NULLIF($17,''),$18,NULLIF($19,''),NULLIF($20,''),NULLIF($21,''),
				        $22,$23,$24,$25,$26,NULLIF($27,''),
				        $28,$29,$30,
				        $31,$32,NULLIF($33,''))
				ON CONFLICT (tenant_id, ac_no, work_date) DO UPDATE SET
				    emp_no = EXCLUDED.emp_no, seq_no = EXCLUDED.seq_no,
				    full_name = EXCLUDED.full_name, auto_assign = EXCLUDED.auto_assign,
				    department = EXCLUDED.department, timetable = EXCLUDED.timetable,
				    on_duty = EXCLUDED.on_duty, off_duty = EXCLUDED.off_duty,
				    clock_in = EXCLUDED.clock_in, clock_out = EXCLUDED.clock_out,
				    normal = EXCLUDED.normal, real_time = EXCLUDED.real_time,
				    late = EXCLUDED.late, early = EXCLUDED.early, absent = EXCLUDED.absent,
				    ot_time = EXCLUDED.ot_time, work_time = EXCLUDED.work_time,
				    exception = EXCLUDED.exception,
				    must_cin = EXCLUDED.must_cin, must_cout = EXCLUDED.must_cout,
				    ndays = EXCLUDED.ndays, weekend = EXCLUDED.weekend, holiday = EXCLUDED.holiday,
				    att_time = EXCLUDED.att_time, ndays_ot = EXCLUDED.ndays_ot,
				    weekend_ot = EXCLUDED.weekend_ot, holiday_ot = EXCLUDED.holiday_ot,
				    checked_in_late = EXCLUDED.checked_in_late,
				    checked_out_early = EXCLUDED.checked_out_early,
				    source_file = EXCLUDED.source_file, imported_at = now()`,
				tenantID, get(row, "emp_no"), acNo, get(row, "seq_no"), get(row, "full_name"),
				get(row, "auto_assign"), get(row, "department"),
				day.Format("2006-01-02"), get(row, "timetable"), onDuty, offDuty, clockIn, clockOut,
				sheetNum(get(row, "normal")), sheetNum(get(row, "real_time")),
				get(row, "late"), get(row, "early"), sheetBool(get(row, "absent")),
				get(row, "ot_time"), get(row, "work_time"), get(row, "exception"),
				sheetBool(get(row, "must_cin")), sheetBool(get(row, "must_cout")),
				sheetNum(get(row, "ndays")), sheetNum(get(row, "weekend")), sheetNum(get(row, "holiday")),
				get(row, "att_time"),
				sheetNum(get(row, "ndays_ot")), sheetNum(get(row, "weekend_ot")), sheetNum(get(row, "holiday_ot")),
				late, early, fileName)
			if execErr != nil {
				skipped++
				if len(problems) < 5 {
					problems = append(problems, fmt.Sprintf("row %d: %s", n+2, execErr.Error()))
				}
				continue
			}
			inserted++
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"inserted": inserted, "skipped": skipped, "errors": problems,
		})
	}
}

// ── the filtered read ────────────────────────────────────────────────────────

type employeeDay struct {
	ACNo       string   `json:"ac_no"`
	EmpNo      string   `json:"emp_no"`
	FullName   string   `json:"full_name"`
	Department string   `json:"department"`
	WorkDate   string   `json:"date"`
	Timetable  string   `json:"timetable"`
	OnDuty     string   `json:"on_duty"`
	OffDuty    string   `json:"off_duty"`
	ClockIn    string   `json:"clock_in"`
	ClockOut   string   `json:"clock_out"`
	Late       string   `json:"late"`
	Early      string   `json:"early"`
	Absent     bool     `json:"absent"`
	OTTime     string   `json:"ot_time"`
	WorkTime   string   `json:"work_time"`
	ATTTime    string   `json:"att_time"`
	Exception  string   `json:"exception"`
	NDays      *float64 `json:"ndays"`
	NDaysOT    *float64 `json:"ndays_ot"`
	LateFlag   bool     `json:"checked_in_late"`
	EarlyFlag  bool     `json:"checked_out_early"`
}

// EmployeeDays is the filtered report. Every filter is optional and they combine, because the
// question is never "show me everybody" — it is "who in Finance was late last week".
func EmployeeDays(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		q := r.URL.Query()

		args := []interface{}{tenantID}
		where := ""
		add := func(clause string, v interface{}) {
			args = append(args, v)
			where += fmt.Sprintf(clause, len(args))
		}

		if v := strings.TrimSpace(q.Get("from")); v != "" {
			add(" AND work_date >= $%d::date", v)
		}
		if v := strings.TrimSpace(q.Get("to")); v != "" {
			add(" AND work_date <= $%d::date", v)
		}
		if v := strings.TrimSpace(q.Get("department")); v != "" {
			add(" AND btrim(lower(COALESCE(department,''))) = btrim(lower($%d))", v)
		}
		// Name and AC-No. are one box on screen: an operator has a person in mind and types
		// whichever identifier they happen to know. Bound once and referenced twice, rather
		// than added twice, so the placeholder numbering cannot drift out of step with args.
		if v := strings.TrimSpace(q.Get("q")); v != "" {
			args = append(args, "%"+strings.ToLower(v)+"%")
			n := len(args)
			where += fmt.Sprintf(
				" AND (btrim(lower(full_name)) LIKE $%d OR btrim(lower(ac_no)) LIKE $%d)", n, n)
		}
		if q.Get("late") == "true" {
			where += " AND checked_in_late"
		}
		if q.Get("early") == "true" {
			where += " AND checked_out_early"
		}
		if q.Get("absent") == "true" {
			where += " AND absent"
		}
		if q.Get("exception") == "true" {
			where += " AND COALESCE(exception,'') <> ''"
		}

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		if err := middleware.SetTenantConn(r.Context(), conn, tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}

		rows, err := conn.Query(r.Context(), `
			SELECT ac_no, COALESCE(emp_no,''), full_name, COALESCE(department,''),
			       to_char(work_date,'YYYY-MM-DD'), COALESCE(timetable,''),
			       COALESCE(on_duty,''), COALESCE(off_duty,''),
			       COALESCE(clock_in,''), COALESCE(clock_out,''),
			       COALESCE(late,''), COALESCE(early,''), absent,
			       COALESCE(ot_time,''), COALESCE(work_time,''), COALESCE(att_time,''),
			       COALESCE(exception,''), ndays, ndays_ot,
			       checked_in_late, checked_out_early
			  FROM employee_attendance_days
			 WHERE tenant_id = $1`+where+`
			 ORDER BY work_date DESC, full_name
			 LIMIT 5000`, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		out := []employeeDay{}
		for rows.Next() {
			var d employeeDay
			if rows.Scan(&d.ACNo, &d.EmpNo, &d.FullName, &d.Department, &d.WorkDate, &d.Timetable,
				&d.OnDuty, &d.OffDuty, &d.ClockIn, &d.ClockOut, &d.Late, &d.Early, &d.Absent,
				&d.OTTime, &d.WorkTime, &d.ATTTime, &d.Exception, &d.NDays, &d.NDaysOT,
				&d.LateFlag, &d.EarlyFlag) == nil {
				out = append(out, d)
			}
		}

		// The departments actually present, so the filter offers real values rather than a free
		// text box that silently matches nothing when it is misspelled.
		depts := []string{}
		if drows, derr := conn.Query(r.Context(), `
			SELECT DISTINCT department FROM employee_attendance_days
			 WHERE tenant_id = $1 AND COALESCE(department,'') <> '' ORDER BY department`, tenantID); derr == nil {
			for drows.Next() {
				var d string
				if drows.Scan(&d) == nil {
					depts = append(depts, d)
				}
			}
			drows.Close()
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"days": out, "departments": depts, "count": len(out),
		})
	}
}

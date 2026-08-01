package handlers

// Rooms / room codes (Phase 4 of the QA subsystem).
//
// `venues` has always been the physical-room table — `venue_id` IS the room code (LR-101) — but the
// only management it had was "list" and "add", and the weekly timetable stored its room as free
// text beside it. That made "LR 101", "LR-101" and "lr101" three different rooms and put per-room
// reporting out of reach. These handlers turn it into a managed registry: full CRUD, bulk
// import/export, a link to the owning school/department, and an active flag (rooms are retired, not
// deleted, because old sessions still point at them).
//
//   GET    /api/v1/admin/tenants/{tenant_id}/rooms
//   POST   /api/v1/admin/tenants/{tenant_id}/rooms
//   PATCH  /api/v1/admin/tenants/{tenant_id}/rooms/{room_code}
//   DELETE /api/v1/admin/tenants/{tenant_id}/rooms/{room_code}
//   POST   /api/v1/admin/tenants/{tenant_id}/rooms/import      (multipart field "roster")
//   GET    /api/v1/admin/tenants/{tenant_id}/rooms/export.xlsx
//   GET    /api/v1/dashboard/rooms                             (picker for the timetable grid)

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

type room struct {
	RoomCode string `json:"room_code"`
	// Mirrors room_code under the column's real name, so callers written against the old
	// /venues shape keep reading the field they expect.
	VenueID      string `json:"venue_id"`
	Name         string `json:"name"`
	Building     string `json:"building"`
	Floor        int    `json:"floor"`
	Capacity     int    `json:"capacity"`
	RoomType     string `json:"room_type"`
	IsActive     bool   `json:"is_active"`
	SchoolID     string `json:"school_id"`
	SchoolName   string `json:"school_name"`
	DepartmentID string `json:"department_id"`
	DeptName     string `json:"department_name"`
	SlotCount    int    `json:"slot_count"` // timetabled sessions using it — the "is this in use?" signal
}

const roomSelect = `
	SELECT v.venue_id, v.name, COALESCE(v.building,''), COALESCE(v.floor,0)::int,
	       COALESCE(v.capacity,0)::int, COALESCE(v.room_type,'LECTURE_HALL'), COALESCE(v.is_active,true),
	       COALESCE(v.school_id::text,''), COALESCE(s.name,''),
	       COALESCE(v.department_id::text,''), COALESCE(d.name,''),
	       (SELECT COUNT(*) FROM timetable_slots ts WHERE ts.venue_id = v.venue_id)::int
	FROM venues v
	LEFT JOIN schools     s ON s.school_id     = v.school_id
	LEFT JOIN departments d ON d.department_id = v.department_id`

func scanRooms(rows pgx.Rows) []room {
	defer rows.Close()
	out := []room{}
	for rows.Next() {
		var v room
		if rows.Scan(&v.RoomCode, &v.Name, &v.Building, &v.Floor, &v.Capacity, &v.RoomType,
			&v.IsActive, &v.SchoolID, &v.SchoolName, &v.DepartmentID, &v.DeptName, &v.SlotCount) == nil {
			v.VenueID = v.RoomCode
			out = append(out, v)
		}
	}
	return out
}

// ListRooms — every room in the tenant, newest structure included. Optional ?school_id=, ?active=.
func ListRooms(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		where := " WHERE v.tenant_id = $1"
		args := []interface{}{tenantID}
		if sid := r.URL.Query().Get("school_id"); sid != "" {
			args = append(args, sid)
			where += fmt.Sprintf(" AND v.school_id = $%d::uuid", len(args))
		}
		if r.URL.Query().Get("active") == "true" {
			where += " AND COALESCE(v.is_active,true)"
		}
		rows, err := adminPool.Query(r.Context(), roomSelect+where+" ORDER BY v.venue_id", args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, scanRooms(rows))
	}
}

type roomInput struct {
	RoomCode string `json:"room_code"`
	// venue_id is what the original Venues form posted; still accepted so the old client keeps
	// working against the new handler.
	VenueID      string  `json:"venue_id"`
	Name         string  `json:"name"`
	Building     *string `json:"building"`
	Floor        *int    `json:"floor"`
	Capacity     *int    `json:"capacity"`
	RoomType     *string `json:"room_type"`
	IsActive     *bool   `json:"is_active"`
	SchoolID     *string `json:"school_id"`
	DepartmentID *string `json:"department_id"`
}

// CreateRoom adds a room. The code is the tenant's own identifier for it (LR-101, LAB2) and becomes
// the key every timetable slot and session points at, so it is normalised to upper case here — the
// one place it is minted — rather than left to whoever types it next.
func CreateRoom(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		var in roomInput
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		code := normalizeRoomCode(firstNonEmpty(in.RoomCode, in.VenueID))
		name := strings.TrimSpace(in.Name)
		if code == "" || name == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "room code and name are required"))
			return
		}
		if len(code) > 50 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "the room code must be 50 characters or fewer"))
			return
		}
		_, err := adminPool.Exec(r.Context(), `
			INSERT INTO venues (venue_id, tenant_id, name, building, floor, capacity, room_type, is_active, school_id, department_id)
			VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,COALESCE(NULLIF($7,''),'LECTURE_HALL'),COALESCE($8,true),
			        NULLIF($9,'')::uuid, NULLIF($10,'')::uuid)`,
			code, tenantID, name, deref(in.Building), derefInt(in.Floor), derefInt(in.Capacity),
			strings.ToUpper(deref(in.RoomType)), in.IsActive, deref(in.SchoolID), deref(in.DepartmentID))
		if err != nil {
			writeJSON(w, http.StatusConflict, errBody("CONFLICT", roomConflictMessage(code, err)))
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"room_code": code, "venue_id": code, "status": "CREATED"})
	}
}

// UpdateRoom is a partial PATCH — only the fields present in the body change. The code itself is
// immutable: it is the foreign key the timetable and every past session hang off.
func UpdateRoom(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		code := chi.URLParam(r, "room_code")
		var in roomInput
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		sets := []string{}
		args := []interface{}{code, tenantID}
		set := func(expr string, val interface{}) {
			args = append(args, val)
			sets = append(sets, fmt.Sprintf(expr, len(args)))
		}
		if strings.TrimSpace(in.Name) != "" {
			set("name = $%d", strings.TrimSpace(in.Name))
		}
		if in.Building != nil {
			set("building = NULLIF($%d,'')", strings.TrimSpace(*in.Building))
		}
		if in.Floor != nil {
			set("floor = $%d", *in.Floor)
		}
		if in.Capacity != nil {
			set("capacity = $%d", *in.Capacity)
		}
		if in.RoomType != nil {
			set("room_type = COALESCE(NULLIF($%d,''),'LECTURE_HALL')", strings.ToUpper(strings.TrimSpace(*in.RoomType)))
		}
		if in.IsActive != nil {
			set("is_active = $%d", *in.IsActive)
		}
		if in.SchoolID != nil {
			set("school_id = NULLIF($%d,'')::uuid", strings.TrimSpace(*in.SchoolID))
		}
		if in.DepartmentID != nil {
			set("department_id = NULLIF($%d,'')::uuid", strings.TrimSpace(*in.DepartmentID))
		}
		if len(sets) == 0 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "nothing to update"))
			return
		}
		tag, err := adminPool.Exec(r.Context(),
			`UPDATE venues SET `+strings.Join(sets, ", ")+` WHERE venue_id = $1 AND tenant_id = $2`, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "no such room in this institution"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"room_code": code, "status": "UPDATED"})
	}
}

// DeleteRoom removes a room that nothing points at. A room already used by the timetable or by past
// sessions is refused with the reason — deactivating it is the right move there, since deleting
// would either fail on the foreign key or orphan history.
func DeleteRoom(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		code := chi.URLParam(r, "room_code")

		var slots, sessions, units int
		_ = adminPool.QueryRow(r.Context(), `
			SELECT (SELECT COUNT(*) FROM timetable_slots WHERE venue_id = $1 AND tenant_id = $2),
			       (SELECT COUNT(*) FROM sessions       WHERE venue_id = $1 AND tenant_id = $2),
			       (SELECT COUNT(*) FROM course_units   WHERE default_venue_id = $1 AND tenant_id = $2)`,
			code, tenantID).Scan(&slots, &sessions, &units)
		if slots+sessions+units > 0 {
			writeJSON(w, http.StatusConflict, errBody("ROOM_IN_USE", fmt.Sprintf(
				"%s is still in use (%d timetable slot(s), %d session(s), %d unit default(s)). Deactivate it instead — it keeps the history intact and hides it from new bookings.",
				code, slots, sessions, units)))
			return
		}
		tag, err := adminPool.Exec(r.Context(),
			`DELETE FROM venues WHERE venue_id = $1 AND tenant_id = $2`, code, tenantID)
		if err != nil {
			writeJSON(w, http.StatusConflict, errBody("ROOM_IN_USE", "that room is referenced elsewhere — deactivate it instead"))
			return
		}
		if tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "no such room in this institution"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "DELETED"})
	}
}

// ─── Bulk import / export ────────────────────────────────────────────────────

var roomExportHeader = []string{
	"room_code", "name", "building", "floor", "capacity", "room_type", "school", "department", "active",
}

// ImportRooms bulk-loads the estates list. Schools/departments are matched by name — an unmatched
// name is reported rather than invented, so a typo cannot quietly create a new school.
func ImportRooms(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "expected multipart/form-data"))
			return
		}
		file, _, err := r.FormFile("roster")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "field 'roster' not found"))
			return
		}
		defer file.Close()
		res, perr := processRoomsUpload(r.Context(), adminPool, tenantID, file)
		if perr != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("CSV_PARSE_ERROR", perr.Error()))
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func processRoomsUpload(ctx context.Context, pool *pgxpool.Pool, tenantID string, src io.Reader) (*importResult, error) {
	rows, idx, err := readTabular(src)
	if err != nil {
		return nil, err
	}
	// Accept the export's own header, plus the obvious synonyms people type.
	alias := map[string]string{"code": "room_code", "venue_id": "room_code", "room": "room_code",
		"room_name": "name", "block": "building", "seats": "capacity", "type": "room_type",
		"college": "school", "faculty": "school", "is_active": "active"}
	for k, v := range alias {
		if i, ok := idx[k]; ok {
			if _, taken := idx[v]; !taken {
				idx[v] = i
			}
		}
	}
	if _, ok := idx["room_code"]; !ok {
		return nil, fmt.Errorf("missing required column: room_code")
	}

	// Name → id lookups, resolved once.
	schools, depts := map[string]string{}, map[string]string{}
	if sr, e := pool.Query(ctx, `SELECT school_id::text, lower(btrim(name)) FROM schools WHERE tenant_id = $1`, tenantID); e == nil {
		for sr.Next() {
			var id, n string
			if sr.Scan(&id, &n) == nil {
				schools[n] = id
			}
		}
		sr.Close()
	}
	if dr, e := pool.Query(ctx, `SELECT department_id::text, lower(btrim(name)) FROM departments WHERE tenant_id = $1`, tenantID); e == nil {
		for dr.Next() {
			var id, n string
			if dr.Scan(&id, &n) == nil {
				depts[n] = id
			}
		}
		dr.Close()
	}

	res := &importResult{Errors: []string{}}
	for ln := 1; ln < len(rows); ln++ {
		get := func(c string) string { return cell(rows[ln], idx, c) }
		code := normalizeRoomCode(get("room_code"))
		if code == "" {
			continue // blank trailing line
		}
		if len(code) > 50 {
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("row %d: room code %q is longer than 50 characters", ln+1, code))
			continue
		}
		name := get("name")
		if name == "" {
			name = code // a code with no separate name is its own name
		}
		schoolID, deptID := "", ""
		if s := get("school"); s != "" {
			if id, ok := schools[strings.ToLower(strings.TrimSpace(s))]; ok {
				schoolID = id
			} else {
				res.Errors = append(res.Errors, fmt.Sprintf("row %d: no school named %q — %s imported without one", ln+1, s, code))
			}
		}
		if d := get("department"); d != "" {
			if id, ok := depts[strings.ToLower(strings.TrimSpace(d))]; ok {
				deptID = id
			} else {
				res.Errors = append(res.Errors, fmt.Sprintf("row %d: no department named %q — %s imported without one", ln+1, d, code))
			}
		}
		active := true
		if a := strings.ToLower(get("active")); a != "" {
			active = a != "no" && a != "false" && a != "0" && a != "n"
		}

		var inserted bool
		err := pool.QueryRow(ctx, `
			INSERT INTO venues (venue_id, tenant_id, name, building, floor, capacity, room_type, is_active, school_id, department_id)
			VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,'')::int,NULLIF($6,'')::int,
			        COALESCE(NULLIF(upper($7),''),'LECTURE_HALL'),$8,NULLIF($9,'')::uuid,NULLIF($10,'')::uuid)
			ON CONFLICT (venue_id) DO UPDATE
			   SET name          = EXCLUDED.name,
			       building      = COALESCE(EXCLUDED.building, venues.building),
			       floor         = COALESCE(EXCLUDED.floor, venues.floor),
			       capacity      = COALESCE(EXCLUDED.capacity, venues.capacity),
			       room_type     = EXCLUDED.room_type,
			       is_active     = EXCLUDED.is_active,
			       school_id     = COALESCE(EXCLUDED.school_id, venues.school_id),
			       department_id = COALESCE(EXCLUDED.department_id, venues.department_id)
			 WHERE venues.tenant_id = $2
			RETURNING (xmax = 0)`,
			code, tenantID, name, get("building"), digitsOnly(get("floor")), digitsOnly(get("capacity")),
			get("room_type"), active, schoolID, deptID).Scan(&inserted)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// The DO UPDATE's tenant guard rejected it: the conflicting row belongs to another
			// institution, because venue_id is a global key.
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("row %d: the room code %s is already taken by another institution — give it a distinct code", ln+1, code))
		case err != nil:
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("row %d: %s", ln+1, err.Error()))
		case inserted:
			res.Inserted++
		default:
			res.Updated++
		}
	}
	return res, nil
}

// ExportRoomsXLSX streams the room registry as a workbook — the same columns the import accepts, so
// an export can be edited and fed straight back in.
func ExportRoomsXLSX(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		rows, err := adminPool.Query(r.Context(), roomSelect+` WHERE v.tenant_id = $1 ORDER BY v.venue_id`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		out := [][]string{roomExportHeader}
		for _, v := range scanRooms(rows) {
			out = append(out, []string{
				v.RoomCode, v.Name, v.Building, blankZero(v.Floor), blankZero(v.Capacity),
				v.RoomType, v.SchoolName, v.DeptName, yesNo(v.IsActive),
			})
		}
		xl, err := buildXLSX(out)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="rooms.xlsx"`)
		_, _ = w.Write(xl)
	}
}

// ─── Picker ──────────────────────────────────────────────────────────────────

// DashboardRooms — the active rooms of the caller's tenant, for the timetable grid's room picker.
// Read-only and tenant-scoped through RLS, so it is safe for every dashboard role.
func DashboardRooms(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
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
			SELECT v.venue_id, v.name, COALESCE(v.building,''), COALESCE(v.capacity,0)::int,
			       COALESCE(v.room_type,'LECTURE_HALL'), COALESCE(s.name,'')
			FROM venues v
			LEFT JOIN schools s ON s.school_id = v.school_id
			WHERE v.tenant_id = $1 AND COALESCE(v.is_active,true)
			ORDER BY v.venue_id`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()
		type pick struct {
			RoomCode string `json:"room_code"`
			Name     string `json:"name"`
			Building string `json:"building"`
			Capacity int    `json:"capacity"`
			RoomType string `json:"room_type"`
			School   string `json:"school"`
		}
		out := []pick{}
		for rows.Next() {
			var p pick
			if rows.Scan(&p.RoomCode, &p.Name, &p.Building, &p.Capacity, &p.RoomType, &p.School) == nil {
				out = append(out, p)
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// resolveVenueSQL matches a free-text room ($7 — the room as typed on the timetable) against the
// managed registry for tenant $1, by code or by name, ignoring case and surrounding space. It is a
// scalar sub-select so it can be dropped straight into an INSERT's value list; an unrecognised room
// simply yields NULL, leaving the free text as the only record of it.
const resolveVenueSQL = `(SELECT v.venue_id FROM venues v
	  WHERE v.tenant_id = $1
	    AND ( btrim(lower(v.venue_id)) = btrim(lower($7)) OR btrim(lower(v.name)) = btrim(lower($7)) )
	  LIMIT 1)`

// ─── Helpers ─────────────────────────────────────────────────────────────────

// normalizeRoomCode collapses the spacing people vary on and upper-cases, so "lr 101" and "LR-101"
// resolve to the same room instead of quietly becoming two.
func normalizeRoomCode(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	return strings.Join(strings.Fields(s), " ")
}

// roomConflictMessage turns the raw unique-violation into something an admin can act on. venue_id
// is a global primary key (a quirk of migration 001), so a clash can also be another institution's.
func roomConflictMessage(code string, err error) string {
	if strings.Contains(err.Error(), "venues_pkey") || strings.Contains(err.Error(), "duplicate key") {
		return fmt.Sprintf("the room code %s is already taken — codes are unique across the platform, so pick a distinct one (e.g. prefix it with the campus)", code)
	}
	return "could not add that room: " + err.Error()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

// digitsOnly keeps a numeric spreadsheet cell that arrived as "12" or "12.0" castable to int, and
// turns anything else into "" so NULLIF drops it rather than failing the row.
func digitsOnly(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return strconv.Itoa(int(f))
	}
	return ""
}

func blankZero(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

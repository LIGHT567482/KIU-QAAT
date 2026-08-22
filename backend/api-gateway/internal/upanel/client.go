package upanel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://kiu.orion13.us"

const (
	KindStudent  = "student"
	KindLecturer = "lecturer"
	KindAdmin    = "admin"
)

const (
	pageSize = 500
	maxPages = 10
)

// Record is one attendance event mapped onto QAAT field names so the same
// screens can show U-Panel data.
//
// U-Panel stores check-ins against an internal roster id (`studentId`). QAAT
// identifies students by registration number (`student_id`) and shows
// full_name, course, unit, study session (Day/Evening/Weekend), year and
// semester. Those are filled from attendance/students + sign-ins + the class list.
type Record struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	PersonID     string `json:"person_id"`
	PersonName   string `json:"person_name"`
	StaffID      string `json:"staff_id"`
	StudentID    string `json:"student_id,omitempty"`
	FullName     string `json:"full_name,omitempty"`
	Present      bool   `json:"present"`
	Verified     bool   `json:"verified"`
	EventType    string `json:"event_type"`
	Course       string `json:"course"`
	Unit         string `json:"unit"`
	UnitName     string `json:"unit_name,omitempty"`
	Session      string `json:"session,omitempty"` // Day / Evening / Weekend / Distance
	Year         string `json:"year"`
	Sem          string `json:"sem"`
	Semester     string `json:"semester,omitempty"`
	Lecturer     string `json:"lecturer"`
	LecturerName string `json:"lecturer_name,omitempty"`
	Room         string `json:"room"`
	Program      string `json:"program"`
	SessionDate  string `json:"session_date,omitempty"`
	Timestamp    string `json:"timestamp"`
	ClosedAt     string `json:"closed_at,omitempty"`
	ListID       string `json:"list_id"`
	SessionID    string `json:"session_id"`
}

// Row is the historical name for a student check-in. Kept so older callers compile.
type Row = Record

type Payload struct {
	Source        string   `json:"source"`
	BaseURL       string   `json:"base_url"`
	Configured    bool     `json:"configured"`
	ListCount     int      `json:"list_count"`
	SessionCount  int      `json:"session_count"`
	RecordCount   int      `json:"record_count"`
	StudentCount  int      `json:"student_count"`
	LecturerCount int      `json:"lecturer_count"`
	AdminCount    int      `json:"admin_count"`
	Stored        int      `json:"stored"`
	Records       []Record `json:"records"`
	FetchedVia    string   `json:"fetched_via"`
	FromCache     bool     `json:"from_cache,omitempty"`
	Message       string   `json:"message,omitempty"`
}

func BaseURL() string {
	u := strings.TrimRight(strings.TrimSpace(os.Getenv("UPANEL_API_URL")), "/")
	if u == "" {
		return defaultBaseURL
	}
	return u
}

func Token() string {
	return strings.TrimSpace(os.Getenv("UPANEL_API_TOKEN"))
}

func Fetch(ctx context.Context) (Payload, error) {
	token := Token()
	if token == "" {
		return Payload{
			Source:     "u-panel",
			BaseURL:    BaseURL(),
			Configured: false,
			Records:    []Record{},
			Message:    "UPANEL_API_TOKEN is not set on the API gateway",
		}, nil
	}
	base := BaseURL()
	client := &http.Client{Timeout: 60 * time.Second}

	lists, sessions, records, via, err := fetchCore(ctx, client, base, token)
	if err != nil {
		return Payload{}, err
	}
	signIns, err := fetchDocs(ctx, client, base, token, "attendance/sign-ins", "signedInAt")
	if err != nil {
		return Payload{}, err
	}
	students, err := fetchDocs(ctx, client, base, token, "attendance/students", "name")
	if err != nil {
		return Payload{}, err
	}
	presence, err := fetchDocs(ctx, client, base, token, "campus/presence", "capturedAt")
	if err != nil {
		return Payload{}, err
	}
	// Lecturer sittings only store lecturerUid (Django user pk) and whoTaught.
	// Staff number lives on accounts/lecturers and accounts/staff-numbers.
	lecturers, err := fetchDocsOptional(ctx, client, base, token, "accounts/lecturers", "fullName")
	if err != nil {
		return Payload{}, err
	}
	staffNums, err := fetchDocsOptional(ctx, client, base, token, "accounts/staff-numbers", "staffNumber")
	if err != nil {
		return Payload{}, err
	}

	ids := studentIdentities(students, signIns)
	lecturerIDs := lecturerIdentities(lecturers, staffNums)
	out := append([]Record{}, joinStudents(lists, sessions, records, ids)...)
	out = append(out, absentSessions(lists, sessions, records, signIns, ids)...)
	out = append(out, joinLecturers(lists, sessions, lecturerIDs)...)
	out = append(out, joinAdmins(presence)...)
	for i := range out {
		out[i].applyQAATFields()
	}
	p := Payload{
		Source:        "u-panel",
		BaseURL:       base,
		Configured:    true,
		ListCount:     len(lists),
		SessionCount:  len(sessions),
		RecordCount:   len(out),
		StudentCount:  countKind(out, KindStudent),
		LecturerCount: countKind(out, KindLecturer),
		AdminCount:    countKind(out, KindAdmin),
		Records:       out,
		FetchedVia:    via,
	}
	return p, nil
}

func fetchCore(ctx context.Context, client *http.Client, base, token string) (lists, sessions, records []map[string]any, via string, err error) {
	if lists, sessions, records, ok, err := fetchExport(ctx, client, base, token); err != nil {
		return nil, nil, nil, "", err
	} else if ok {
		return lists, sessions, records, "export", nil
	}
	lists, err = fetchDocs(ctx, client, base, token, "attendance/lists", "createdAt")
	if err != nil {
		return nil, nil, nil, "", err
	}
	sessions, err = fetchDocs(ctx, client, base, token, "attendance/sessions", "startTime")
	if err != nil {
		return nil, nil, nil, "", err
	}
	records, err = fetchDocs(ctx, client, base, token, "attendance/records", "timestamp")
	if err != nil {
		return nil, nil, nil, "", err
	}
	return lists, sessions, records, "collections", nil
}

func fetchExport(ctx context.Context, client *http.Client, base, token string) (lists, sessions, records []map[string]any, ok bool, err error) {
	q := url.Values{}
	q.Set("limit", "5000")
	status, body, err := get(ctx, client, base+"/api/attendance/export/", token, q)
	if err != nil {
		return nil, nil, nil, false, err
	}
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
		return nil, nil, nil, false, nil
	}
	if status < 200 || status >= 300 {
		return nil, nil, nil, false, fmt.Errorf("u-panel export: HTTP %d: %s", status, clip(body, 240))
	}
	var raw struct {
		Lists    []map[string]any `json:"lists"`
		Sessions []map[string]any `json:"sessions"`
		Records  []map[string]any `json:"records"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, nil, nil, false, fmt.Errorf("u-panel export: decode: %w", err)
	}
	return raw.Lists, raw.Sessions, raw.Records, true, nil
}

func fetchDocs(ctx context.Context, client *http.Client, base, token, collection, orderField string) ([]map[string]any, error) {
	var all []map[string]any
	cursor := ""
	seen := map[string]bool{}
	for page := 0; page < maxPages; page++ {
		q := url.Values{}
		q.Set("limit", strconv.Itoa(pageSize))
		if orderField != "" {
			q.Set("ordering", "-"+orderField)
			if cursor != "" {
				q.Set(orderField+"__lt", cursor)
			}
		}
		status, body, err := get(ctx, client, base+"/api/"+collection+"/", token, q)
		if err != nil {
			return nil, err
		}
		if status == http.StatusNotFound {
			return []map[string]any{}, nil
		}
		if status < 200 || status >= 300 {
			return nil, fmt.Errorf("u-panel %s: HTTP %d: %s", collection, status, clip(body, 240))
		}
		var docs []map[string]any
		if err := json.Unmarshal(body, &docs); err != nil {
			return nil, fmt.Errorf("u-panel %s: decode: %w", collection, err)
		}
		added := 0
		for _, d := range docs {
			id := field(d, "id")
			if id != "" && seen[id] {
				continue
			}
			if id != "" {
				seen[id] = true
			}
			all = append(all, d)
			added++
		}
		if len(docs) < pageSize || added == 0 {
			break
		}
		last := field(docs[len(docs)-1], orderField, "id")
		if last == "" || last == cursor {
			break
		}
		cursor = last
	}
	return all, nil
}

// fetchDocsOptional treats 401/403 the same as a missing collection so a
// read-only attendance token still returns sittings when the accounts
// directory is out of scope.
func fetchDocsOptional(ctx context.Context, client *http.Client, base, token, collection, orderField string) ([]map[string]any, error) {
	docs, err := fetchDocs(ctx, client, base, token, collection, orderField)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "HTTP 401") || strings.Contains(msg, "HTTP 403") {
			return []map[string]any{}, nil
		}
		return nil, err
	}
	return docs, nil
}

func get(ctx context.Context, client *http.Client, rawURL, token string, q url.Values) (int, []byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0, nil, err
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return 0, nil, err
	}
	return res.StatusCode, body, nil
}

func joinStudents(lists, sessions, records []map[string]any, ids map[string]identity) []Record {
	listByID := indexByID(lists)
	sessionByID := indexByID(sessions)
	out := make([]Record, 0, len(records))
	for _, rec := range records {
		sessionID := field(rec, "sessionId", "session_id")
		listID := field(rec, "listId", "list_id")
		sess := sessionByID[sessionID]
		if listID == "" {
			listID = field(sess, "listId", "list_id")
		}
		list := listByID[listID]
		if boolField(sess, "rollDiscarded") || boolField(sess, "roll_discarded") {
			continue
		}
		rawID := field(rec, "studentId", "student_id", "registrationNumber", "registration_number")
		reg, name := resolveStudent(rawID, ids)
		name = firstNonEmpty(name, field(rec, "studentName", "student_name", "fullName", "full_name"))
		present := true
		if _, ok := rec["present"]; ok {
			present = boolField(rec, "present")
		}
		when := firstNonEmpty(field(rec, "timestamp", "capturedAt", "created_at"), field(sess, "startTime", "start_time"))
		out = append(out, Record{
			ID:         firstNonEmpty(field(rec, "id"), sessionID+"_"+rawID),
			Kind:       KindStudent,
			PersonID:   firstNonEmpty(reg, rawID),
			PersonName: name,
			StudentID:  firstNonEmpty(reg, rawID),
			FullName:   name,
			Present:    present,
			Verified:   boolField(rec, "verified"),
			EventType:  presentLabel(present),
			Course:     firstNonEmpty(field(rec, "course"), firstCourse(list)),
			Timestamp:  when,
			ListID:     listID,
			SessionID:  sessionID,
			Lecturer:   field(list, "whoTaught", "who_taught"),
			Room:       field(list, "room"),
			Unit:       field(list, "courseUnitName", "course_unit_name"),
			Year:       field(list, "year"),
			Sem:        field(list, "sem"),
			Program:    field(list, "program"),
		})
	}
	return out
}

// absentSessions emits one ABSENT student row for every completed sitting that an enrolled
// student did not check in to. U-Panel often only writes present check-ins until the lecturer
// device finalizes the roll, so the API copy would otherwise hide missed sessions.
func absentSessions(lists, sessions, records, signIns []map[string]any, ids map[string]identity) []Record {
	listByID := indexByID(lists)
	seen := map[string]bool{}
	for _, rec := range records {
		sess := field(rec, "sessionId", "session_id")
		stu := field(rec, "studentId", "student_id", "registrationNumber", "registration_number")
		if sess != "" && stu != "" {
			seen[sess+"|"+stu] = true
		}
	}
	enrolled := map[string][]map[string]string{}
	for _, si := range signIns {
		listID := field(si, "listId", "list_id")
		stu := field(si, "studentId", "student_id", "registrationNumber", "registration_number")
		if listID == "" || stu == "" {
			continue
		}
		reg, name := resolveStudent(stu, ids)
		name = firstNonEmpty(name, field(si, "studentName", "student_name", "fullName", "full_name"))
		enrolled[listID] = append(enrolled[listID], map[string]string{
			"id":       stu,
			"reg":      reg,
			"name":     name,
			"course":   field(si, "course"),
			"signedAt": field(si, "signedInAt", "signed_in_at", "timestamp"),
		})
	}
	out := []Record{}
	for _, sess := range sessions {
		if boolField(sess, "rollDiscarded") || boolField(sess, "roll_discarded") {
			continue
		}
		if !sessionCompleted(sess) {
			continue
		}
		sessionID := field(sess, "id")
		listID := field(sess, "listId", "list_id")
		if sessionID == "" || listID == "" {
			continue
		}
		list := listByID[listID]
		when := firstNonEmpty(field(sess, "endTime", "end_time"), field(sess, "startTime", "start_time"))
		endAt := parseTime(field(sess, "endTime", "end_time"))
		for _, e := range enrolled[listID] {
			key := sessionID + "|" + e["id"]
			if seen[key] {
				continue
			}
			if joined := parseTime(e["signedAt"]); joined != nil && endAt != nil && endAt.Before(*joined) {
				continue
			}
			seen[key] = true
			out = append(out, Record{
				ID:         "absent:" + sessionID + "_" + e["id"],
				Kind:       KindStudent,
				PersonID:   firstNonEmpty(e["reg"], e["id"]),
				PersonName: e["name"],
				StudentID:  firstNonEmpty(e["reg"], e["id"]),
				FullName:   e["name"],
				Present:    false,
				EventType:  "ABSENT",
				Course:     firstNonEmpty(e["course"], firstCourse(list)),
				Timestamp:  when,
				ListID:     listID,
				SessionID:  sessionID,
				Lecturer:   field(list, "whoTaught", "who_taught"),
				Room:       field(list, "room"),
				Unit:       field(list, "courseUnitName", "course_unit_name"),
				Year:       field(list, "year"),
				Sem:        field(list, "sem"),
				Program:    field(list, "program"),
			})
		}
	}
	return out
}

func sessionCompleted(sess map[string]any) bool {
	status := strings.ToLower(field(sess, "status"))
	if status == "closed" || status == "ended" || status == "complete" || status == "completed" {
		return true
	}
	if end := parseTime(field(sess, "endTime", "end_time")); end != nil && time.Now().After(*end) {
		return true
	}
	return false
}

func joinLecturers(lists, sessions []map[string]any, ids map[string]lecturerIdent) []Record {
	listByID := indexByID(lists)
	out := make([]Record, 0, len(sessions)+len(lists))
	listedSessions := map[string]bool{}
	for _, sess := range sessions {
		sessionID := field(sess, "id")
		listID := field(sess, "listId", "list_id")
		list := listByID[listID]
		listedSessions[listID] = true
		staff, name := resolveLecturerFrom(list, sess, ids)
		start := field(sess, "startTime", "start_time", "createdAt", "created_at")
		end := field(sess, "endTime", "end_time")
		out = append(out, Record{
			ID:           "lecturer:session:" + firstNonEmpty(sessionID, listID+"_"+start),
			Kind:         KindLecturer,
			PersonID:     staff,
			PersonName:   name,
			FullName:     name,
			StaffID:      staff,
			LecturerName: name,
			Present:      true,
			EventType:    "LECTURE",
			Course:       firstNonEmpty(field(list, "courseUnitName", "course_unit_name"), field(sess, "course")),
			Timestamp:    start,
			ClosedAt:     end,
			ListID:       listID,
			SessionID:    sessionID,
			Lecturer:     name,
			Room:         field(list, "room"),
			Unit:         field(list, "courseUnitName", "course_unit_name"),
			Year:         field(list, "year"),
			Sem:          field(list, "sem"),
			Program:      field(list, "program"),
		})
	}
	for _, list := range lists {
		signed := field(list, "lecturerSignedAt", "lecturer_signed_at")
		if signed == "" {
			continue
		}
		listID := field(list, "id")
		if listedSessions[listID] {
			continue
		}
		staff, name := resolveLecturerFrom(list, nil, ids)
		out = append(out, Record{
			ID:           "lecturer:list:" + listID,
			Kind:         KindLecturer,
			PersonID:     staff,
			PersonName:   name,
			FullName:     name,
			StaffID:      staff,
			LecturerName: name,
			Present:      true,
			EventType:    "LECTURER_SIGN",
			Course:       field(list, "courseUnitName", "course_unit_name"),
			Timestamp:    signed,
			ListID:       listID,
			Lecturer:     name,
			Room:         field(list, "room"),
			Unit:         field(list, "courseUnitName", "course_unit_name"),
			Year:         field(list, "year"),
			Sem:          field(list, "sem"),
			Program:      field(list, "program"),
		})
	}
	return out
}

func joinAdmins(events []map[string]any) []Record {
	out := make([]Record, 0, len(events))
	for _, ev := range events {
		kind := strings.ToLower(field(ev, "kind"))
		eventType := "PUNCH"
		switch kind {
		case "arrival":
			eventType = "IN"
		case "departure":
			eventType = "OUT"
		}
		uid := field(ev, "adminUid", "admin_uid", "userId", "user_id")
		name := field(ev, "displayName", "display_name", "fullName", "full_name")
		staff := firstNonEmpty(field(ev, "staffNumber", "staff_number", "staffId", "staff_id"), uid)
		title := field(ev, "jobTitle", "job_title")
		out = append(out, Record{
			ID:         "admin:" + firstNonEmpty(field(ev, "id"), uid+"_"+field(ev, "capturedAt", "captured_at")),
			Kind:       KindAdmin,
			PersonID:   staff,
			PersonName: name,
			FullName:   name,
			StaffID:    staff,
			Present:    true,
			EventType:  eventType,
			Timestamp:  field(ev, "capturedAt", "captured_at", "timestamp"),
			Room:       field(ev, "label", "campus"),
			Course:     title,
			Unit:       title,
		})
	}
	return out
}

func indexByID(docs []map[string]any) map[string]map[string]any {
	out := make(map[string]map[string]any, len(docs))
	for _, d := range docs {
		id := field(d, "id")
		if id != "" {
			out[id] = d
		}
	}
	return out
}

func field(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if s := strings.TrimSpace(t); s != "" && s != "<nil>" {
					return s
				}
			case float64:
				if t == float64(int64(t)) {
					return strconv.FormatInt(int64(t), 10)
				}
				return strconv.FormatFloat(t, 'f', -1, 64)
			case json.Number:
				return t.String()
			case bool:
				if t {
					return "true"
				}
				return "false"
			}
		}
	}
	return ""
}

func boolField(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key]
	if !ok {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "true" || s == "1" || s == "yes"
	case float64:
		return t != 0
	default:
		return false
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

type identity struct {
	reg  string
	name string
}

func studentIdentities(students, signIns []map[string]any) map[string]identity {
	out := map[string]identity{}
	put := func(key, reg, name string) {
		key = strings.TrimSpace(key)
		if key == "" {
			return
		}
		cur := out[key]
		if cur.reg == "" {
			cur.reg = strings.TrimSpace(reg)
		}
		if cur.name == "" {
			cur.name = strings.TrimSpace(name)
		}
		out[key] = cur
	}
	for _, s := range students {
		id := field(s, "id")
		reg := strings.ToUpper(field(s, "registrationNumber", "registration_number"))
		name := field(s, "name", "fullName", "full_name")
		put(id, reg, name)
		put(reg, reg, name)
		put(strings.ToUpper(id), reg, name)
	}
	for _, si := range signIns {
		id := field(si, "studentId", "student_id")
		reg := strings.ToUpper(field(si, "registrationNumber", "registration_number"))
		name := field(si, "studentName", "student_name", "fullName", "full_name")
		if reg == "" {
			reg = strings.ToUpper(id)
		}
		put(id, reg, name)
		put(reg, reg, name)
		put(strings.ToUpper(id), reg, name)
	}
	return out
}

func resolveStudent(rawID string, ids map[string]identity) (reg, name string) {
	rawID = strings.TrimSpace(rawID)
	if rawID == "" {
		return "", ""
	}
	if ident, ok := ids[rawID]; ok {
		return firstNonEmpty(ident.reg, rawID), ident.name
	}
	if ident, ok := ids[strings.ToUpper(rawID)]; ok {
		return firstNonEmpty(ident.reg, rawID), ident.name
	}
	return rawID, ""
}

type lecturerIdent struct {
	staff string
	name  string
	uid   string
}

func normalizeStaff(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func normPersonName(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func lecturerIdentities(lecturers, staffNums []map[string]any) map[string]lecturerIdent {
	out := map[string]lecturerIdent{}
	put := func(key, staff, name, uid string) {
		key = strings.TrimSpace(key)
		if key == "" {
			return
		}
		cur := out[key]
		if cur.staff == "" {
			cur.staff = normalizeStaff(staff)
		}
		if cur.name == "" {
			cur.name = strings.TrimSpace(name)
		}
		if cur.uid == "" {
			cur.uid = strings.TrimSpace(uid)
		}
		out[key] = cur
	}
	index := func(uid, staff, name string) {
		staff = normalizeStaff(staff)
		uid = strings.TrimSpace(uid)
		name = strings.TrimSpace(name)
		put(uid, staff, name, uid)
		put(strings.ToUpper(uid), staff, name, uid)
		if staff != "" {
			put(staff, staff, name, uid)
		}
		if n := normPersonName(name); n != "" {
			put(n, staff, name, uid)
		}
	}
	for _, d := range lecturers {
		index(
			field(d, "id", "uid", "userId", "user_id"),
			field(d, "staffNumber", "staff_number", "staffId", "staff_id"),
			field(d, "fullName", "full_name", "name"),
		)
	}
	for _, d := range staffNums {
		index(
			field(d, "uid", "userId", "user_id"),
			firstNonEmpty(field(d, "staffNumber", "staff_number", "staffId", "staff_id"), field(d, "id")),
			field(d, "fullName", "full_name", "name"),
		)
	}
	return out
}

func resolveLecturer(uid, name string, ids map[string]lecturerIdent) (staff, display string) {
	uid = strings.TrimSpace(uid)
	name = strings.TrimSpace(name)
	lookup := func(key string) (lecturerIdent, bool) {
		if key == "" {
			return lecturerIdent{}, false
		}
		if ident, ok := ids[key]; ok {
			return ident, true
		}
		if ident, ok := ids[strings.ToUpper(key)]; ok {
			return ident, true
		}
		return lecturerIdent{}, false
	}
	ident, ok := lookup(uid)
	if !ok {
		ident, ok = lookup(normalizeStaff(uid))
	}
	if !ok {
		ident, ok = lookup(normPersonName(name))
	}
	if ok {
		return firstNonEmpty(ident.staff, uid, name), firstNonEmpty(ident.name, name)
	}
	return firstNonEmpty(uid, name), name
}

func resolveLecturerFrom(list, sess map[string]any, ids map[string]lecturerIdent) (staff, name string) {
	name = firstNonEmpty(field(list, "whoTaught", "who_taught"), field(sess, "createdByName", "created_by_name"))
	uid := firstNonEmpty(
		field(list, "lecturerUid", "lecturer_uid"),
		field(sess, "createdBy", "created_by"),
	)
	listed := firstNonEmpty(
		field(list, "staffNumber", "staff_number", "lecturerStaffNumber", "lecturer_staff_number", "staffId", "staff_id"),
		field(sess, "staffNumber", "staff_number", "staffId", "staff_id"),
	)
	staff, name = resolveLecturer(uid, name, ids)
	staff = firstNonEmpty(normalizeStaff(listed), staff)
	name = firstNonEmpty(name, field(list, "whoTaught", "who_taught"))
	return staff, name
}

func firstCourse(list map[string]any) string {
	if list == nil {
		return ""
	}
	if v, ok := list["courses"]; ok {
		switch t := v.(type) {
		case []any:
			if len(t) > 0 {
				if s := strings.TrimSpace(fmt.Sprint(t[0])); s != "" && s != "<nil>" {
					return s
				}
			}
		case []string:
			if len(t) > 0 {
				if s := strings.TrimSpace(t[0]); s != "" {
					return s
				}
			}
		}
	}
	return field(list, "courseUnitName", "course_unit_name")
}

func mapProgram(p string) string {
	s := strings.TrimSpace(p)
	if s == "" {
		return ""
	}
	switch strings.ToLower(s) {
	case "evening":
		return "Evening"
	case "weekend":
		return "Weekend"
	case "day", "morning":
		return "Day"
	case "distance", "online", "remote":
		return "Distance"
	default:
		return s
	}
}

func (r *Record) applyQAATFields() {
	r.FullName = firstNonEmpty(r.FullName, r.PersonName)
	r.PersonName = firstNonEmpty(r.PersonName, r.FullName)
	session := mapProgram(firstNonEmpty(r.Session, r.Program))
	r.Session = session
	r.Program = session
	r.Semester = firstNonEmpty(r.Semester, r.Sem)
	r.Sem = r.Semester
	r.LecturerName = firstNonEmpty(r.LecturerName, r.Lecturer, r.PersonName)
	if r.Kind == KindLecturer {
		r.Lecturer = firstNonEmpty(r.Lecturer, r.LecturerName, r.PersonName)
		r.LecturerName = r.Lecturer
	} else {
		r.Lecturer = firstNonEmpty(r.LecturerName, r.Lecturer)
		r.LecturerName = r.Lecturer
	}
	r.UnitName = firstNonEmpty(r.UnitName, r.Unit)
	r.Unit = r.UnitName
	if r.Kind == KindStudent {
		r.StudentID = firstNonEmpty(r.StudentID, r.PersonID)
		r.PersonID = r.StudentID
	}
	if r.Kind == KindLecturer || r.Kind == KindAdmin {
		r.StaffID = firstNonEmpty(normalizeStaff(r.StaffID), r.PersonID)
		r.PersonID = firstNonEmpty(r.StaffID, r.PersonID)
	}
	if r.SessionDate == "" {
		if t := parseTime(r.Timestamp); t != nil {
			loc := kampalaLoc()
			r.SessionDate = t.In(loc).Format("2006-01-02")
		}
	}
}

func kampalaLoc() *time.Location {
	loc, err := time.LoadLocation("Africa/Kampala")
	if err != nil {
		return time.UTC
	}
	return loc
}

func presentLabel(present bool) string {
	if present {
		return "PRESENT"
	}
	return "ABSENT"
}

func countKind(rows []Record, kind string) int {
	n := 0
	for _, r := range rows {
		if r.Kind == kind {
			n++
		}
	}
	return n
}

func clip(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

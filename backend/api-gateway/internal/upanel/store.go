package upanel

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Upsert writes fetched U-Panel rows into QAAT. Re-fetching the same document updates
// denormalized fields in place rather than duplicating the event.
func Upsert(ctx context.Context, pool *pgxpool.Pool, rows []Record) (int, error) {
	if pool == nil || len(rows) == 0 {
		return 0, nil
	}
	batch := &pgx.Batch{}
	const q = `
		INSERT INTO upanel_attendance (
			kind, external_id, person_id, person_name, staff_id, present, event_type,
			occurred_at, closed_at, list_id, session_id, course, unit_name, lecturer,
			room, year_label, semester, program, imported_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18, now()
		)
		ON CONFLICT (kind, external_id) DO UPDATE SET
			person_id   = EXCLUDED.person_id,
			person_name = EXCLUDED.person_name,
			staff_id    = EXCLUDED.staff_id,
			present     = EXCLUDED.present,
			event_type  = EXCLUDED.event_type,
			occurred_at = EXCLUDED.occurred_at,
			closed_at   = EXCLUDED.closed_at,
			list_id     = EXCLUDED.list_id,
			session_id  = EXCLUDED.session_id,
			course      = EXCLUDED.course,
			unit_name   = EXCLUDED.unit_name,
			lecturer    = EXCLUDED.lecturer,
			room        = EXCLUDED.room,
			year_label  = EXCLUDED.year_label,
			semester    = EXCLUDED.semester,
			program     = EXCLUDED.program,
			imported_at = now()`
	n := 0
	for _, r := range rows {
		id := strings.TrimSpace(r.ID)
		kind := strings.TrimSpace(r.Kind)
		if id == "" || kind == "" {
			continue
		}
		personID := firstNonEmpty(r.StudentID, r.StaffID, r.PersonID)
		personName := firstNonEmpty(r.FullName, r.PersonName)
		unit := firstNonEmpty(r.Unit, r.UnitName)
		lecturer := firstNonEmpty(r.LecturerName, r.Lecturer)
		sem := firstNonEmpty(r.Semester, r.Sem)
		program := firstNonEmpty(r.Session, r.Program)
		batch.Queue(q,
			kind, id, personID, personName, r.StaffID, r.Present, r.EventType,
			parseTime(r.Timestamp), parseTime(r.ClosedAt), r.ListID, r.SessionID, r.Course, unit, lecturer,
			r.Room, r.Year, sem, program,
		)
		n++
	}
	if n == 0 {
		return 0, nil
	}
	br := pool.SendBatch(ctx, batch)
	defer br.Close()
	for i := 0; i < n; i++ {
		if _, err := br.Exec(); err != nil {
			return i, err
		}
	}
	return n, nil
}

// List returns stored U-Panel rows, newest first. kind may be student|lecturer|admin or empty.
func List(ctx context.Context, pool *pgxpool.Pool, kind string) ([]Record, error) {
	if pool == nil {
		return nil, nil
	}
	q := `
		SELECT external_id, kind, person_id, person_name, staff_id, present, event_type,
		       occurred_at, closed_at, list_id, session_id, course, unit_name, lecturer,
		       room, year_label, semester, program
		FROM upanel_attendance`
	args := []any{}
	if k := strings.TrimSpace(kind); k != "" {
		q += " WHERE kind = $1"
		args = append(args, k)
	}
	q += " ORDER BY occurred_at DESC NULLS LAST, imported_at DESC"
	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		var r Record
		var occurred, closed *time.Time
		if err := rows.Scan(
			&r.ID, &r.Kind, &r.PersonID, &r.PersonName, &r.StaffID, &r.Present, &r.EventType,
			&occurred, &closed, &r.ListID, &r.SessionID, &r.Course, &r.Unit, &r.Lecturer,
			&r.Room, &r.Year, &r.Sem, &r.Program,
		); err != nil {
			return nil, err
		}
		if r.Kind == KindStudent {
			r.StudentID = r.PersonID
		}
		if occurred != nil {
			r.Timestamp = occurred.UTC().Format(time.RFC3339)
		}
		if closed != nil {
			r.ClosedAt = closed.UTC().Format(time.RFC3339)
		}
		r.applyQAATFields()
		out = append(out, r)
	}
	return out, rows.Err()
}

func parseTime(raw string) *time.Time {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return &t
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if n > 1e12 {
			t := time.UnixMilli(n)
			return &t
		}
		if n > 1e9 {
			t := time.Unix(n, 0)
			return &t
		}
	}
	return nil
}

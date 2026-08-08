package handlers

// One coordinator, two taps at once.
//
// OpenSession refuses a second live session for a coordinator who already has one, and that
// refusal used to be a SELECT followed by an INSERT with nothing between them: two requests that
// arrived together both read "none open" and both inserted. A stress run of 20 simultaneous opens
// produced 2 sessions. The cost is not a stray row — it is one lecture split across two rosters,
// each with its own room code, so half the hall checks into the session the coordinator is not
// looking at, and their attendance is filed against a session nobody closes.
//
// This is database-backed, because the defect lives in the interleaving of two connections and a
// mock cannot have that. Like the rest of the repo's integration tests it SKIPS unless a
// connection is supplied:
//
//	DB_URL='postgres://qaat:pw@localhost:5434/qaat?sslmode=disable' go test ./internal/handlers/ -run Race -v

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

func racePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DB_URL")
	if url == "" {
		t.Skip("set DB_URL to run the session-open race test")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil || pool.Ping(context.Background()) != nil {
		t.Skipf("database unavailable: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestOpenSession_ConcurrentOpensCreateOnlyOne(t *testing.T) {
	pool := racePool(t)
	ctx := context.Background()

	// The test brings its own course, unit and coordinator, so it never depends on what the
	// institution happens to teach or on a session someone else left open.
	const (
		unitID   = "RACETEST-UNIT"
		courseID = "RACETEST-COURSE"
		email    = "racetest.coordinator@racetest.invalid"
	)

	var tenantID string
	if err := pool.QueryRow(ctx,
		`SELECT tenant_id::text FROM tenants
		  WHERE tenant_id <> '00000000-0000-0000-0000-000000000000'
		  ORDER BY created_at LIMIT 1`).Scan(&tenantID); err != nil {
		t.Skipf("no institution in this database: %v", err)
	}

	// OPEN THE SESSION WINDOW FOR THE DURATION, then put it back exactly as it was.
	//
	// OpenSession refuses outside the institution's lecture hours, and that refusal is also a 409 —
	// so run in the evening this test saw twenty conflicts and could say nothing about the race it
	// exists to catch. Skipping after 17:00 would have been worse than useless: a guard against a
	// data-corrupting race that quietly stops guarding for most of the day is one nobody notices
	// has stopped.
	var savedStart, savedEnd string
	var savedDays []int16
	_ = pool.QueryRow(ctx, `
		SELECT to_char(session_window_start,'HH24:MI'), to_char(session_window_end,'HH24:MI'),
		       session_active_days
		  FROM tenants WHERE tenant_id = $1`, tenantID).Scan(&savedStart, &savedEnd, &savedDays)

	cleanup := func() {
		pool.Exec(ctx, `DELETE FROM lecturer_attendance_logs WHERE unit_id = $1`, unitID) //nolint:errcheck
		pool.Exec(ctx, `DELETE FROM sessions WHERE unit_id = $1`, unitID)                 //nolint:errcheck
		pool.Exec(ctx, `DELETE FROM course_units WHERE unit_id = $1`, unitID)             //nolint:errcheck
		pool.Exec(ctx, `DELETE FROM courses WHERE course_id = $1`, courseID)              //nolint:errcheck
		pool.Exec(ctx, `DELETE FROM users WHERE email = $1`, email)                       //nolint:errcheck
		if savedStart != "" {
			pool.Exec(ctx, `
				UPDATE tenants SET session_window_start = $2::time, session_window_end = $3::time,
				                   session_active_days = $4
				 WHERE tenant_id = $1`, tenantID, savedStart, savedEnd, savedDays) //nolint:errcheck
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := pool.Exec(ctx, `
		UPDATE tenants SET session_window_start = '00:00'::time, session_window_end = '23:59'::time,
		                   session_active_days = ARRAY[1,2,3,4,5,6,7]::smallint[]
		 WHERE tenant_id = $1`, tenantID); err != nil {
		t.Skipf("cannot widen the session window for the test: %v", err)
	}

	var coordID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active)
		VALUES ($1, $2, 'x', 'COORDINATOR', 'Race Test Coordinator', true)
		RETURNING user_id::text`, tenantID, email).Scan(&coordID); err != nil {
		t.Skipf("cannot seed a coordinator (needs the owner role): %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO courses (course_id, tenant_id, name) VALUES ($1, $2, 'Race Test Course')`,
		courseID, tenantID); err != nil {
		t.Fatalf("seed course: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO course_units (unit_id, tenant_id, course_id, name)
		 VALUES ($1, $2, $3, 'Race Test Unit')`, unitID, tenantID, courseID); err != nil {
		t.Fatalf("seed unit: %v", err)
	}

	handler := OpenSession(pool)
	body := fmt.Sprintf(`{"unit_id":%q}`, unitID)

	const attempts = 20
	codes := make([]int, attempts)
	errCodes := make([]string, attempts)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/open", strings.NewReader(body))
			c := context.WithValue(req.Context(), middleware.CtxTenantID, tenantID)
			c = context.WithValue(c, middleware.CtxUserID, coordID)
			c = context.WithValue(c, middleware.CtxRole, middleware.RoleCoordinator)
			req = req.WithContext(c)
			rec := httptest.NewRecorder()
			<-start // release them together, so they contend rather than queue
			handler(rec, req)
			codes[i] = rec.Code
			var body struct {
				Error string `json:"error"`
			}
			_ = json.Unmarshal(rec.Body.Bytes(), &body)
			errCodes[i] = body.Error
		}(i)
	}
	close(start)
	wg.Wait()

	created, conflicts, windowShut := 0, 0, 0
	other := map[int]int{}
	for i, c := range codes {
		switch {
		case c == http.StatusCreated:
			created++
		// TWO different refusals share the 409, and conflating them made this test fail every
		// evening: WINDOW_CLOSED means the institution is not running sessions at this hour and
		// nobody could have opened one, while SESSION_ALREADY_OPEN is the race actually under
		// test. Counting the first as evidence of the second turned "it is 8pm" into a red build.
		case c == http.StatusConflict && errCodes[i] == "WINDOW_CLOSED":
			windowShut++
		case c == http.StatusConflict:
			conflicts++
		default:
			other[c]++
		}
	}

	if created == 0 && conflicts == 0 {
		t.Skipf("no session could be opened at all (window-closed=%d, other=%v) — outside the institution's session hours",
			windowShut, other)
	}

	if created != 1 {
		t.Errorf("%d simultaneous opens created %d sessions, want exactly 1 (conflicts=%d other=%v)",
			attempts, created, conflicts, other)
	}

	var open int
	pool.QueryRow(ctx, //nolint:errcheck
		`SELECT count(*) FROM sessions
		  WHERE unit_id = $1 AND session_status IN ('ACTIVE','PENDING_LECTURER')`, unitID).Scan(&open)
	if open != 1 {
		t.Errorf("%d live sessions left in the database, want exactly 1", open)
	}
}

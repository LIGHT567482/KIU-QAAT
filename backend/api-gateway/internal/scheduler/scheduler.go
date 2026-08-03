// Package scheduler runs the gateway's timed jobs.
//
// The hard requirement is not "run every minute" — it is "never silently skip".
// These services are on Render's free plan and free services sleep after ~15
// minutes idle. A naive ticker wakes up, sees that it is now 14:00, fires the
// 14:00 work and never learns that 12:00 and 13:00 also needed doing. For a
// reminder system that is the worst failure mode available: the lecturer was
// promised a nudge, no nudge arrived, and nothing anywhere recorded that.
//
// So a job is not "a thing that runs now". A job is a function over a *window of
// time*, and the scheduler's contract is that every window between the last
// success and now gets passed to it exactly once. The watermark lives in
// scheduled_job_runs, so it survives a restart, a redeploy and a sleep.
//
// The price is that a job may be handed a window it has already seen (a crash
// between doing the work and moving the watermark). Jobs are therefore required
// to be idempotent, which they get from notification_log's
// UNIQUE (tenant_id, kind, subject_key, subject_date) — see MarkSent.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/clock"
)

// Window is a half-open interval [From, To) of civil time that a job must
// consider. A job is given consecutive, non-overlapping windows.
type Window struct {
	From time.Time
	To   time.Time
}

// JobFunc does the work for one window. Returning an error stops the watermark
// advancing past this window, so the next sweep retries it.
type JobFunc func(ctx context.Context, w Window) error

// Job is a registered unit of scheduled work.
type Job struct {
	// Name is the primary key in scheduled_job_runs. Changing it resets the
	// watermark, which replays history — so don't, casually.
	Name string
	// Every is the window length. A job with Every=time.Minute is handed one
	// window per minute of wall time, however long the process was asleep.
	Every time.Duration
	// MaxCatchUp bounds the replay after a long outage. A gateway down for a week
	// should not send seven days of "your lecture starts in 10 minutes". Windows
	// older than this are skipped and counted, not processed.
	MaxCatchUp time.Duration
	Run        JobFunc
}

// Scheduler owns the ticker and the watermarks.
type Scheduler struct {
	pool   *pgxpool.Pool
	log    *slog.Logger
	jobs   []Job
	tick   time.Duration
	cancel context.CancelFunc
}

func New(pool *pgxpool.Pool, log *slog.Logger) *Scheduler {
	return &Scheduler{pool: pool, log: log, tick: time.Minute}
}

// Register adds a job. Call before Start.
func (s *Scheduler) Register(j Job) {
	if j.Every <= 0 {
		j.Every = time.Minute
	}
	if j.MaxCatchUp <= 0 {
		j.MaxCatchUp = 6 * time.Hour
	}
	s.jobs = append(s.jobs, j)
}

// Start begins ticking in the background. Stop cancels it.
func (s *Scheduler) Start(ctx context.Context) {
	if len(s.jobs) == 0 {
		s.log.Info("scheduler: no jobs registered, not starting")
		return
	}
	ctx, s.cancel = context.WithCancel(ctx)
	names := make([]string, 0, len(s.jobs))
	for _, j := range s.jobs {
		names = append(names, j.Name)
	}
	s.log.Info("scheduler starting", "jobs", names, "tick", s.tick.String(), "timezone", clock.Name())

	go func() {
		t := time.NewTicker(s.tick)
		defer t.Stop()
		// Sweep once on boot rather than waiting a full tick: a redeploy is
		// exactly when catch-up matters most.
		s.SweepAll(ctx)
		for {
			select {
			case <-ctx.Done():
				s.log.Info("scheduler stopped")
				return
			case <-t.C:
				s.SweepAll(ctx)
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// SweepAll runs every job up to now. Exported so the /internal/cron endpoints can
// drive the same code path — an external pinger and the in-process ticker must
// never be two different implementations that drift apart.
func (s *Scheduler) SweepAll(ctx context.Context) {
	for _, j := range s.jobs {
		if err := s.sweep(ctx, j); err != nil {
			s.log.Error("scheduler job failed", "job", j.Name, "error", err)
		}
	}
}

// SweepOne runs a single named job. Returns false if no such job is registered.
func (s *Scheduler) SweepOne(ctx context.Context, name string) (bool, error) {
	for _, j := range s.jobs {
		if j.Name == name {
			return true, s.sweep(ctx, j)
		}
	}
	return false, nil
}

// maxWindowsPerSweep is a safety valve. A misconfigured Every (say, one second)
// after a long sleep would otherwise spin for a very long time holding a database
// connection.
const maxWindowsPerSweep = 10_000

// Plan is the catch-up arithmetic: given where a job got to and what time it is
// now, which windows still need processing?
//
// Pure, and separated from sweep, because this is the part that has to be right.
// Everything else in this package is plumbing; if this function is wrong the
// system either spams people or silently drops their reminders, and both are
// invisible until someone complains.
//
// Returns the windows to run in order, and how many were dropped for being older
// than maxCatchUp.
func Plan(last, now time.Time, every, maxCatchUp time.Duration) (windows []Window, skipped int) {
	if every <= 0 {
		return nil, 0
	}
	// A gateway down for a week must not wake up and send seven days of "your
	// lecture starts in 10 minutes". Old windows are dropped, not processed.
	if maxCatchUp > 0 {
		if cutoff := now.Add(-maxCatchUp); last.Before(cutoff) {
			skipped = int(cutoff.Sub(last) / every)
			last = cutoff
		}
	}
	for cursor := last; !cursor.Add(every).After(now); cursor = cursor.Add(every) {
		windows = append(windows, Window{From: cursor, To: cursor.Add(every)})
		if len(windows) >= maxWindowsPerSweep {
			break
		}
	}
	return windows, skipped
}

// sweep processes every elapsed window for one job and advances its watermark.
func (s *Scheduler) sweep(ctx context.Context, j Job) error {
	started := time.Now()
	now := clock.Now()

	last, err := s.watermark(ctx, j, now)
	if err != nil {
		return err
	}

	windows, skipped := Plan(last, now, j.Every, j.MaxCatchUp)
	if skipped > 0 {
		s.log.Warn("scheduler dropped stale windows", "job", j.Name, "skipped", skipped,
			"max_catch_up", j.MaxCatchUp.String())
	}

	processed := 0
	cursor := last
	if len(windows) > 0 {
		cursor = windows[0].From
	}
	for _, w := range windows {
		if err := j.Run(ctx, w); err != nil {
			// Leave the watermark where it is: the next sweep retries this window.
			s.record(ctx, j.Name, cursor, "ERROR", err.Error(), time.Since(started), processed)
			return fmt.Errorf("window %s..%s: %w", w.From.Format(time.RFC3339), w.To.Format(time.RFC3339), err)
		}
		cursor = w.To
		processed++
	}

	if processed > 0 || skipped > 0 {
		s.log.Info("scheduler swept", "job", j.Name, "windows", processed, "skipped_stale", skipped,
			"took_ms", time.Since(started).Milliseconds())
	}
	return s.record(ctx, j.Name, cursor, "OK", "", time.Since(started), processed)
}

// watermark reads where the job got to, seeding a first run at one window back so
// a brand-new job does not immediately replay all of history.
func (s *Scheduler) watermark(ctx context.Context, j Job, now time.Time) (time.Time, error) {
	var last time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT last_run_at FROM scheduled_job_runs WHERE job_name = $1`, j.Name).Scan(&last)
	if err != nil {
		seed := now.Add(-j.Every)
		if _, ierr := s.pool.Exec(ctx,
			`INSERT INTO scheduled_job_runs (job_name, last_run_at) VALUES ($1, $2)
			 ON CONFLICT (job_name) DO NOTHING`, j.Name, seed); ierr != nil {
			return time.Time{}, fmt.Errorf("seed watermark: %w", ierr)
		}
		return seed, nil
	}
	return last.In(clock.Location()), nil
}

func (s *Scheduler) record(ctx context.Context, name string, at time.Time, status, errMsg string, took time.Duration, windows int) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE scheduled_job_runs
		    SET last_run_at = $2, last_status = $3, last_error = NULLIF($4,''),
		        last_duration_ms = $5, windows_caught_up = $6, updated_at = now()
		  WHERE job_name = $1`,
		name, at, status, errMsg, took.Milliseconds(), windows)
	return err
}

// AlreadySent reports whether this exact notification has gone out, and claims it
// if not. It is the idempotency gate every job must pass before sending.
//
// Claim-then-send, not send-then-record: a crash between the two should result in
// a notification that was recorded but not delivered, never one delivered twice.
// A missing nudge is recoverable by the person; a duplicate 3am alert erodes
// trust in every alert after it.
func AlreadySent(ctx context.Context, pool *pgxpool.Pool, tenantID, kind, subjectKey string, subjectDate time.Time, recipient string, channels string) (bool, error) {
	var inserted bool
	err := pool.QueryRow(ctx, `
		INSERT INTO notification_log (tenant_id, kind, subject_key, subject_date, recipient_user_id, channels)
		VALUES ($1, $2, $3, $4::date, NULLIF($5,'')::uuid, $6)
		ON CONFLICT (tenant_id, kind, subject_key, subject_date) DO NOTHING
		RETURNING true`,
		tenantID, kind, subjectKey, subjectDate.Format("2006-01-02"), recipient, channels).Scan(&inserted)
	if err != nil {
		// No row returned means the ON CONFLICT fired — already sent.
		if err.Error() == "no rows in result set" {
			return true, nil
		}
		return false, err
	}
	return !inserted, nil
}

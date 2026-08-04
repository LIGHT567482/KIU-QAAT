package jobs

// Employee attendance alerts.
//
//   not checked in    — the shift started and no clock-in was recorded
//   checked in late   — clock-in was after on-duty
//   check-out reminder— the shift is ending; don't forget to clock out
//   left early        — clock-out was before off-duty
//
// DELIVERY. The in-app inbox is the wrong channel here and only here: employees are
// non-teaching staff with no dashboard account (migration 047 — they are managed by
// the ADMIN and identified by a badge number, not a login). So these go out over
// email and WhatsApp through the notification-service, which already has both
// transports; they were simply unreachable from anything but the no-show report.
//
// TIMING. The sheet is uploaded after the fact, so "you have not checked in" cannot
// fire from it in real time. It fires from what the sheet says once uploaded, which
// is what makes it useful the following morning rather than at 08:05 — and the
// late/early alerts, which are inherently retrospective, are the ones the request
// actually turns on. The check-in/check-out reminders run on the clock instead, off
// the shift times the sheet has already established for that person.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/clock"
	"github.com/qaat/api-gateway/internal/scheduler"
)

const (
	KindEmployeeLate       = "EMPLOYEE_LATE"
	KindEmployeeEarlyOut   = "EMPLOYEE_EARLY_OUT"
	KindEmployeeNoCheckin  = "EMPLOYEE_NO_CHECKIN"
	KindEmployeeCheckout   = "EMPLOYEE_CHECKOUT_REMINDER"
	checkoutReminderHHMM   = "17:00" // when to remind people to clock out
	employeeAlertBatchSize = 500
)

// employeeAlert is one person to contact about one day.
type employeeAlert struct {
	TenantID   string
	TenantName string
	ACNo       string
	Name       string
	Email      string
	Phone      string
	Department string
	Date       string
	OnDuty     string
	OffDuty    string
	ClockIn    string
	ClockOut   string
}

// RegisterEmployeeJobs wires the employee alerts into the scheduler.
//
// Five-minute windows rather than one-minute: none of these is anchored to a
// precise instant the way a lecture reminder is, and a wider window is five times
// fewer sweeps on a service that sleeps. MaxCatchUp is a working day, so a gateway
// down overnight still tells people what it found when it wakes.
func RegisterEmployeeJobs(s *scheduler.Scheduler, pool *pgxpool.Pool) {
	s.Register(scheduler.Job{
		Name: "employee_exceptions", Every: 5 * minute, MaxCatchUp: 12 * 60 * minute,
		Run: func(ctx context.Context, w scheduler.Window) error { return employeeExceptions(ctx, pool, w) },
	})
	s.Register(scheduler.Job{
		Name: "employee_checkout_reminder", Every: 5 * minute, MaxCatchUp: 2 * 60 * minute,
		Run: func(ctx context.Context, w scheduler.Window) error { return checkoutReminder(ctx, pool, w) },
	})
}

// employeeExceptions tells people what their own record says: no check-in, a late
// arrival, an early departure. One pass over the days imported since the last sweep,
// so an upload of last month does not re-alert a month of history — only rows whose
// import is newer than the watermark are considered.
func employeeExceptions(ctx context.Context, pool *pgxpool.Pool, w scheduler.Window) error {
	rows, err := pool.Query(ctx, `
		SELECT d.tenant_id::text, COALESCE(t.name,''), d.ac_no, d.full_name,
		       COALESCE(e.email,''), COALESCE(e.phone,''), COALESCE(d.department,''),
		       to_char(d.work_date,'YYYY-MM-DD'),
		       COALESCE(d.on_duty,''), COALESCE(d.off_duty,''),
		       COALESCE(d.clock_in,''), COALESCE(d.clock_out,''),
		       d.checked_in_late, d.checked_out_early, d.absent
		  FROM employee_attendance_days d
		  JOIN tenants t ON t.tenant_id = d.tenant_id
		  -- The registry carries the contact details; the sheet only carries the badge
		  -- number. No email and no phone means nobody to tell, so those rows are skipped
		  -- rather than counted as delivered.
		  LEFT JOIN employees e ON e.tenant_id = d.tenant_id
		                       AND btrim(lower(e.staff_id)) = btrim(lower(d.ac_no))
		 WHERE d.imported_at >= $1 AND d.imported_at < $2
		   AND (d.checked_in_late OR d.checked_out_early OR d.absent)
		   AND (COALESCE(e.email,'') <> '' OR COALESCE(e.phone,'') <> '')
		 LIMIT $3`, w.From, w.To, employeeAlertBatchSize)
	if err != nil {
		return fmt.Errorf("employee exceptions: %w", err)
	}
	defer rows.Close()

	type pending struct {
		a                     employeeAlert
		late, early, absentee bool
	}
	var batch []pending
	for rows.Next() {
		var p pending
		if rows.Scan(&p.a.TenantID, &p.a.TenantName, &p.a.ACNo, &p.a.Name, &p.a.Email, &p.a.Phone,
			&p.a.Department, &p.a.Date, &p.a.OnDuty, &p.a.OffDuty, &p.a.ClockIn, &p.a.ClockOut,
			&p.late, &p.early, &p.absentee) == nil {
			batch = append(batch, p)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, p := range batch {
		switch {
		case p.absentee:
			if err := alertEmployee(ctx, pool, p.a, KindEmployeeNoCheckin,
				"No check-in recorded",
				fmt.Sprintf("Our records show no check-in for you on %s. If you were at work, please see HR so the record can be corrected.", p.a.Date)); err != nil {
				return err
			}
		case p.late:
			if err := alertEmployee(ctx, pool, p.a, KindEmployeeLate,
				"Late check-in recorded",
				fmt.Sprintf("You checked in at %s on %s, after your %s start.", p.a.ClockIn, p.a.Date, p.a.OnDuty)); err != nil {
				return err
			}
		}
		// Early departure is reported alongside a late arrival rather than instead of it:
		// they are two separate facts about the day and suppressing one would leave a
		// record the person never saw.
		if p.early {
			if err := alertEmployee(ctx, pool, p.a, KindEmployeeEarlyOut,
				"Early check-out recorded",
				fmt.Sprintf("You checked out at %s on %s, before your %s finish.", p.a.ClockOut, p.a.Date, p.a.OffDuty)); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkoutReminder nudges everyone who checked in today but has not checked out, as
// the working day ends. Fires once, in the window containing 17:00.
func checkoutReminder(ctx context.Context, pool *pgxpool.Pool, w scheduler.Window) error {
	if _, ok := hhmmWithin(w, checkoutReminderHHMM); !ok {
		return nil
	}

	rows, err := pool.Query(ctx, `
		SELECT d.tenant_id::text, COALESCE(t.name,''), d.ac_no, d.full_name,
		       COALESCE(e.email,''), COALESCE(e.phone,''), COALESCE(d.department,''),
		       to_char(d.work_date,'YYYY-MM-DD'),
		       COALESCE(d.on_duty,''), COALESCE(d.off_duty,''),
		       COALESCE(d.clock_in,''), COALESCE(d.clock_out,'')
		  FROM employee_attendance_days d
		  JOIN tenants t ON t.tenant_id = d.tenant_id
		  LEFT JOIN employees e ON e.tenant_id = d.tenant_id
		                       AND btrim(lower(e.staff_id)) = btrim(lower(d.ac_no))
		 WHERE d.work_date = CURRENT_DATE
		   AND COALESCE(d.clock_in,'')  <> ''
		   AND COALESCE(d.clock_out,'') =  ''
		   AND (COALESCE(e.email,'') <> '' OR COALESCE(e.phone,'') <> '')
		 LIMIT $1`, employeeAlertBatchSize)
	if err != nil {
		return fmt.Errorf("checkout reminder: %w", err)
	}
	defer rows.Close()

	var batch []employeeAlert
	for rows.Next() {
		var a employeeAlert
		if rows.Scan(&a.TenantID, &a.TenantName, &a.ACNo, &a.Name, &a.Email, &a.Phone,
			&a.Department, &a.Date, &a.OnDuty, &a.OffDuty, &a.ClockIn, &a.ClockOut) == nil {
			batch = append(batch, a)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, a := range batch {
		if err := alertEmployee(ctx, pool, a, KindEmployeeCheckout,
			"Remember to check out",
			fmt.Sprintf("Your shift ends at %s. Please remember to check out before you leave — an unclosed day is recorded as incomplete.", a.OffDuty)); err != nil {
			return err
		}
	}
	return nil
}

// alertEmployee claims the notification, then sends it. Claim-first for the same
// reason as everywhere else in this package: a duplicate is worse than a miss.
func alertEmployee(ctx context.Context, pool *pgxpool.Pool, a employeeAlert, kind, subject, body string) error {
	day, err := clock.ParseDate(a.Date)
	if err != nil {
		day = clock.Now()
	}
	// The subject key is the badge number, so one person gets one of each kind per day
	// however many times the sheet is re-uploaded.
	already, err := scheduler.AlreadySent(ctx, pool, a.TenantID, kind, a.ACNo, day, "", "EMAIL,WHATSAPP")
	if err != nil {
		return fmt.Errorf("claim %s for %s: %w", kind, a.ACNo, err)
	}
	if already {
		return nil
	}
	// Delivery failure is logged by the notification-service and deliberately does not
	// fail the sweep: one unreachable address must not stop the other 499 alerts.
	postDirectNotification(ctx, a, subject, body)
	return nil
}

// postDirectNotification hands one message to the notification-service for email and
// WhatsApp delivery.
func postDirectNotification(ctx context.Context, a employeeAlert, subject, body string) {
	base := strings.TrimRight(os.Getenv("NOTIFICATION_URL"), "/")
	if base == "" {
		base = "http://notification-service:3004"
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"tenant":  a.TenantName,
		"subject": subject,
		"message": body,
		"recipients": []map[string]string{{
			"name": a.Name, "email": a.Email, "phone": a.Phone, "department": a.Department,
		}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/notify/direct", bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	if resp, err := client.Do(req); err == nil {
		_ = resp.Body.Close()
	}
}

// hhmmWithin reports whether a wall-clock time falls inside this window — used to
// fire a once-a-day job in whichever sweep contains it, including a catch-up sweep
// that arrives late.
func hhmmWithin(w scheduler.Window, hhmm string) (time.Time, bool) {
	var h, m int
	if _, err := fmt.Sscanf(hhmm, "%d:%d", &h, &m); err != nil {
		return time.Time{}, false
	}
	target := time.Date(w.From.Year(), w.From.Month(), w.From.Day(), h, m, 0, 0, w.From.Location())
	if !target.Before(w.From) && target.Before(w.To) {
		return target, true
	}
	return time.Time{}, false
}

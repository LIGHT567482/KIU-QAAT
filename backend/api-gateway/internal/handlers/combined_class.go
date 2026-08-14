package handlers

// THE COMBINED-CLASS CODE, over HTTP.
//
// The handshake itself is meant to happen with no network at all: the lecturer's phone derives the
// three digits, reads them out, and each coordinator's phone derives the same digits and compares.
// Nothing here is on that path, and nothing here is required for a lecture to run in a hall with no
// signal. See internal/checkin/combinedclass.go and the Kotlin CombinedClassCode for the offline
// mechanism these two endpoints mirror.
//
// So what are they for? Three things the phones cannot do for themselves:
//
//   - A lecturer whose own handset has failed, or who is teaching from a borrowed phone, can still
//     be told today's code by the web console.
//   - A coordinator can confirm a code they were given when they doubt they heard it correctly,
//     rather than opening a register on a guess.
//   - The server has to reach the SAME answer as the phones when packages sync, and an endpoint
//     that exercises the shared derivation is how that stays honest in a running system rather than
//     only in a unit test.
//
// The key is the tenant's student_hash_key — already shipped to every coordinator phone inside the
// session manifest, which is precisely why the phones can derive offline. It is never returned by
// these endpoints; only the three digits it produces.

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/checkin"
	"github.com/qaat/api-gateway/internal/clock"
	"github.com/qaat/api-gateway/internal/middleware"
)

// sharedClassKey loads the institution's offline derivation key. Returned as bytes rather than a
// string so it is never accidentally logged or serialised alongside the code it produced.
func sharedClassKey(r *http.Request, pool *pgxpool.Pool, tenantID string) ([]byte, bool) {
	var k *string
	if pool.QueryRow(r.Context(),
		`SELECT student_hash_key FROM tenants WHERE tenant_id = $1`, tenantID).Scan(&k) != nil {
		return nil, false
	}
	if k == nil || *k == "" {
		return nil, false
	}
	return []byte(*k), true
}

// classKeyFromQuery builds the combined-class identity from the slot the caller names. Deliberately
// the same derivation the phones use — room, weekday and start time — so a class key never has to
// be stored, transmitted or kept in step between the two.
func classKeyFromQuery(r *http.Request) (string, bool) {
	room := strings.TrimSpace(r.URL.Query().Get("room"))
	start := strings.TrimSpace(r.URL.Query().Get("start"))
	day, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("day")))
	if room == "" || start == "" || err != nil || day < 1 || day > 7 {
		return "", false
	}
	return checkin.CombinedClassKey(room, day, start), true
}

// GET /api/v1/lecturer/combined-class-code?room=&day=&start=
//
// The lecturer's own code for today. Scoped to the caller: the lecturer_id in the derivation comes
// from the signed-in account, never from a parameter, so this cannot be used to look up the code
// another lecturer would read out.
func LecturerCombinedClassCode(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())

		lecturerID, ok := resolveLecturerID(adminPool, r, tenantID, userID)
		if !ok {
			writeJSON(w, http.StatusForbidden, errBody("NOT_A_LECTURER",
				"this account is not linked to a lecturer record"))
			return
		}
		classKey, ok := classKeyFromQuery(r)
		if !ok {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST",
				"room, day (1-7) and start (HH:MM) identify the lecture and are all required"))
			return
		}
		key, ok := sharedClassKey(r, adminPool, tenantID)
		if !ok {
			writeJSON(w, http.StatusInternalServerError, errBody("NO_HASH_KEY",
				"the institution has no derivation key set — codes cannot be issued"))
			return
		}

		// The institution's own date, not UTC: an evening lecture is already tomorrow in UTC, and
		// deriving from that would hand out a code no coordinator's phone agrees with.
		today := clock.Today()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"code":        checkin.CombinedClassCode(key, lecturerID, classKey, today),
			"valid_for":   today,
			"digits":      checkin.CombinedClassDigits,
			"read_it_out": "Give this to the other coordinators in the room. It works only today.",
		})
	}
}

// POST /api/v1/coordinator/combined-class-code/verify
//
// Body: {code, lecturer_id, room, day, start}
//
// Answers only "is this the code for that lecture today". It does NOT open a register: a
// coordinator's phone opens its own register offline once its own check succeeds, and making that
// depend on a round trip would defeat the point of deriving the code in the first place.
func VerifyCombinedClassCode(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		var req struct {
			Code       string `json:"code"`
			LecturerID string `json:"lecturer_id"`
			Room       string `json:"room"`
			Day        int    `json:"day"`
			Start      string `json:"start"`
		}
		if err := decodeJSON(r, &req); err != nil || req.Code == "" || req.LecturerID == "" ||
			strings.TrimSpace(req.Room) == "" || req.Day < 1 || req.Day > 7 || req.Start == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST",
				"code, lecturer_id, room, day (1-7) and start (HH:MM) are all required"))
			return
		}
		key, ok := sharedClassKey(r, adminPool, tenantID)
		if !ok {
			writeJSON(w, http.StatusInternalServerError, errBody("NO_HASH_KEY",
				"the institution has no derivation key set — codes cannot be checked"))
			return
		}

		today := clock.Today()
		classKey := checkin.CombinedClassKey(req.Room, req.Day, req.Start)
		valid := checkin.ValidateCombinedClassCode(key, req.Code, req.LecturerID, classKey, today)

		// A wrong code is answered 200-with-false, not 4xx. It is a normal outcome of a coordinator
		// mishearing three digits across a lecture hall, and a client that had to distinguish "you
		// typed it wrong" from "the request was malformed" by status code would get it wrong.
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"valid":     valid,
			"valid_for": today,
			"message": map[bool]string{
				true:  "Code accepted — the lecturer has started this lecture.",
				false: "That is not today's code for this lecture. Ask the lecturer to read it again.",
			}[valid],
		})
	}
}

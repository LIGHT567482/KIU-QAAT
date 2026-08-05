// storm — a load harness for the moment thousands of students touch QAAT at once.
//
// Why this exists next to the k6 scripts: k6 has to be installed, and the scripts beside it are
// written against placeholder tokens, so they have never actually run against this stack. This is
// a single `go run` against the live gateway with real seeded registration numbers, which means a
// number produced here is a measurement rather than an intention.
//
//	go run ./tests/load/storm -base https://localhost:8443 -n 5000 -c 500 -scenario progress
//
// Scenarios model what a cohort actually does simultaneously:
//
//	progress  GET /api/v1/student/progress?reg=…  — every student checking their attendance,
//	          which is what happens the hour results or eligibility are announced.
//	login     POST /api/v1/auth/app-login         — a cohort opening the app at the start of a
//	          lecture. Rate-limited per source IP by design; on campus Wi-Fi a whole cohort
//	          shares ONE public IP, so this is where shedding shows up.
//	health    GET /health                         — the floor. Any latency here is the harness,
//	          the proxy or the machine, not the application, and it calibrates the rest.
//
// It reports the status distribution as well as latency, because on a rate-limited endpoint the
// interesting answer is not "how fast" but "how many students were refused" — an error rate that
// a latency percentile alone would hide entirely.
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type result struct {
	status  int // HTTP status, or 0 for a transport failure
	latency time.Duration
	err     string
	retries int
}

func main() {
	var (
		base     = flag.String("base", "https://localhost:8443", "gateway base URL")
		n        = flag.Int("n", 5000, "total requests (one per student)")
		conc     = flag.Int("c", 500, "how many are in flight at once")
		scenario = flag.String("scenario", "progress", "progress | login | health")
		prefix   = flag.String("prefix", "LOAD-", "seeded registration-number prefix")
		org      = flag.String("org", "", "institution slug for login")
		password = flag.String("password", "Student1234!", "password for the login scenario")
		timeout  = flag.Duration("timeout", 30*time.Second, "per-request timeout")
		warmup   = flag.Int("warmup", 20, "requests to discard before measuring (fills pools/caches)")
		retries  = flag.Int("retries", 0, "on 429, honour Retry-After and try again up to this many times (0 = give up, which is what a client that ignores the header does)")
	)
	flag.Parse()

	// The stack terminates TLS with a self-signed certificate (infra/certs). Skipping
	// verification is correct HERE and nowhere else: this is a load generator pointed at a
	// host the operator named on the command line, not a client trusting a stranger.
	transport := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // local load target
		MaxIdleConns:        *conc * 2,
		MaxIdleConnsPerHost: *conc * 2,
		MaxConnsPerHost:     0,
		IdleConnTimeout:     90 * time.Second,
	}
	client := &http.Client{Transport: transport, Timeout: *timeout}

	fire := requestFor(*scenario, *base, *prefix, *org, *password)
	if fire == nil {
		fmt.Fprintf(os.Stderr, "unknown scenario %q\n", *scenario)
		os.Exit(2)
	}

	// Warm-up, discarded. The first requests pay for TLS handshakes, connection-pool growth and
	// a cold query plan; folding those into the measurement makes a p99 that describes the
	// harness starting up rather than the system under load.
	for i := 0; i < *warmup; i++ {
		if r, err := fire(i); err == nil {
			resp, e := client.Do(r)
			if e == nil {
				io.Copy(io.Discard, resp.Body) //nolint:errcheck
				resp.Body.Close()
			}
		}
	}

	fmt.Printf("storm: %s — %d requests, %d concurrent, against %s\n", *scenario, *n, *conc, *base)

	results := make([]result, *n)
	var inFlight, peak int64

	// Every worker blocks on the same gate and is released together, so the load arrives as a
	// burst — a lecture ending, not a trickle. A ramp would measure a system that had time to
	// warm into the load, which is the opposite of the question being asked.
	gate := make(chan struct{})
	sem := make(chan struct{}, *conc)
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < *n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-gate
			sem <- struct{}{}
			defer func() { <-sem }()

			cur := atomic.AddInt64(&inFlight, 1)
			for {
				old := atomic.LoadInt64(&peak)
				if cur <= old || atomic.CompareAndSwapInt64(&peak, old, cur) {
					break
				}
			}
			defer atomic.AddInt64(&inFlight, -1)

			// One student's whole attempt, including any backoff. The latency recorded is
			// what THEY waited, not what the last HTTP round trip took — a 429 followed by a
			// successful retry two seconds later is a two-second wait for a real person, and
			// reporting only the final 200 would hide the cost of shedding entirely.
			t0 := time.Now()
			var res result
			for attempt := 0; ; attempt++ {
				req, err := fire(i)
				if err != nil {
					res = result{err: err.Error(), retries: attempt}
					break
				}
				resp, err := client.Do(req)
				if err != nil {
					res = result{err: trimErr(err), retries: attempt}
					break
				}
				io.Copy(io.Discard, resp.Body) //nolint:errcheck
				code, retryAfter := resp.StatusCode, resp.Header.Get("Retry-After")
				resp.Body.Close()

				if code == http.StatusTooManyRequests && attempt < *retries {
					time.Sleep(backoff(retryAfter, attempt))
					continue
				}
				res = result{status: code, retries: attempt}
				break
			}
			res.latency = time.Since(t0)
			results[i] = res
		}(i)
	}

	close(gate)
	wg.Wait()
	elapsed := time.Since(start)

	report(*scenario, results, elapsed, int(atomic.LoadInt64(&peak)))
}

// backoff is how long a well-behaved client waits after a 429: the server's own Retry-After when
// it sent one, otherwise a doubling delay. The jitter matters — without it every refused student
// retries in the same instant and rebuilds the stampede that caused the refusal.
func backoff(retryAfter string, attempt int) time.Duration {
	base := 2 * time.Second
	if retryAfter != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && secs > 0 {
			base = time.Duration(secs) * time.Second
		}
	}
	d := base << uint(min(attempt, 3))
	return d + time.Duration(rand.Int63n(int64(d/2)))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// trimErr strips the URL Go's transport errors repeat verbatim. Thousands of failures printed in
// full are thousands of copies of the target address with the one differing clause buried at the
// end, which is exactly the part worth grouping on.
func trimErr(err error) string {
	s := err.Error()
	if strings.HasPrefix(s, "Get \"") || strings.HasPrefix(s, "Post \"") {
		if j := strings.LastIndex(s, "\": "); j > 0 {
			return s[j+3:]
		}
	}
	return s
}

func requestFor(scenario, base, prefix, org, password string) func(int) (*http.Request, error) {
	ctx := context.Background()
	switch scenario {
	case "progress":
		return func(i int) (*http.Request, error) {
			reg := fmt.Sprintf("%s%06d", prefix, i+1)
			url := fmt.Sprintf("%s/api/v1/student/progress?reg=%s", base, reg)
			return http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		}
	case "login":
		return func(i int) (*http.Request, error) {
			body, _ := json.Marshal(map[string]string{
				"identifier": fmt.Sprintf("%s%06d", prefix, i+1),
				"password":   password,
				"org":        org,
			})
			req, err := http.NewRequestWithContext(ctx, http.MethodPost,
				base+"/api/v1/auth/app-login", strings.NewReader(string(body)))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", "application/json")
			return req, nil
		}
	case "health":
		return func(int) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, http.MethodGet, base+"/health", nil)
		}
	}
	return nil
}

func report(scenario string, rs []result, elapsed time.Duration, peak int) {
	byStatus := map[int]int{}
	byErr := map[string]int{}
	lat := make([]time.Duration, 0, len(rs))
	var ok, failed, retried, retries int

	for _, r := range rs {
		if r.retries > 0 {
			retried++
			retries += r.retries
		}
		if r.err != "" {
			failed++
			byErr[r.err]++
			continue
		}
		byStatus[r.status]++
		lat = append(lat, r.latency)
		if r.status >= 200 && r.status < 300 {
			ok++
		}
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })

	fmt.Printf("\n── %s ──────────────────────────────────────────\n", scenario)
	fmt.Printf("requests        %d\n", len(rs))
	fmt.Printf("wall clock      %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("throughput      %.0f req/s\n", float64(len(rs))/elapsed.Seconds())
	fmt.Printf("peak in flight  %d\n", peak)
	fmt.Printf("2xx             %d (%.1f%%)\n", ok, pct(ok, len(rs)))
	if failed > 0 {
		fmt.Printf("transport fails %d (%.1f%%)\n", failed, pct(failed, len(rs)))
	}
	if retried > 0 {
		fmt.Printf("backed off      %d students (%.1f%%), %d retries total\n",
			retried, pct(retried, len(rs)), retries)
	}

	fmt.Printf("\nstatus\n")
	codes := make([]int, 0, len(byStatus))
	for c := range byStatus {
		codes = append(codes, c)
	}
	sort.Ints(codes)
	for _, c := range codes {
		fmt.Printf("  %-3d           %6d  (%.1f%%)  %s\n", c, byStatus[c], pct(byStatus[c], len(rs)), meaning(c))
	}
	if len(byErr) > 0 {
		fmt.Printf("\ntransport errors\n")
		for e, n := range byErr {
			fmt.Printf("  %6d  %s\n", n, e)
		}
	}

	if len(lat) > 0 {
		fmt.Printf("\nlatency (answered requests)\n")
		fmt.Printf("  min           %s\n", lat[0].Round(time.Millisecond))
		fmt.Printf("  p50           %s\n", quantile(lat, 0.50).Round(time.Millisecond))
		fmt.Printf("  p95           %s\n", quantile(lat, 0.95).Round(time.Millisecond))
		fmt.Printf("  p99           %s\n", quantile(lat, 0.99).Round(time.Millisecond))
		fmt.Printf("  max           %s\n", lat[len(lat)-1].Round(time.Millisecond))
	}
	fmt.Println()
}

func meaning(code int) string {
	switch code {
	case 200:
		return "answered"
	case 400:
		return "bad request"
	case 401:
		return "unauthenticated"
	case 404:
		return "no such student — check the seeded prefix"
	case 429:
		return "RATE LIMITED — these students were refused"
	case 500, 502, 503, 504:
		return "SERVER ERROR — the system buckled"
	}
	return ""
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) * 100 / float64(b)
}

func quantile(sorted []time.Duration, q float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	i := int(math.Ceil(q*float64(len(sorted)))) - 1
	if i < 0 {
		i = 0
	}
	if i >= len(sorted) {
		i = len(sorted) - 1
	}
	return sorted[i]
}

# Load tests

Two surfaces carry a stampede, and they fail differently, so they are measured separately.

| Surface | Harness | What it answers |
|---|---|---|
| The coordinator's phone (in-room hub) | `frontend/coordinator-android/app/src/test/kotlin/InRoomStressTest.kt` | Can one handset take a hall full of check-ins without dropping or double-counting anyone? |
| The cloud gateway | `tests/load/storm` | What happens when the institution touches the API at once — and how many students are *refused*? |

The `k6-*.js` scripts predate both. They need k6 installed and are written against placeholder
tokens, so they have never run against this stack; treat them as sketches, not as coverage.

## The in-room hub

```bash
cd frontend/coordinator-android
./gradlew :app:testDebugUnitTest --tests '*InRoomStressTest'
./gradlew :app:testDebugUnitTest --tests '*InRoomStressTest' \
  -Dqaat.stress.students=5000 -Dqaat.stress.concurrency=250   # dial it up
```

It boots the real `InRoomServer` on a free port and checks in over the reg-number path — the one
that ships. It asserts that every student who submits is recorded, exactly once, with its own
sequence number (that ordering is what the sealed package uploaded to the server is built on), and
prints throughput and latency so a regression shows up in the log.

Measured on this machine, 5000 students / 250 concurrent: **5000 recorded, 0 dropped, 0
duplicates**, 235 check-ins/s, p50 903ms, p95 1.9s.

## The cloud gateway

```bash
# 1. give it a population — the seeded database has five students
docker exec -i infra-postgres-1 psql -U qaat -d qaat -v n=5000 -f - < tests/load/seed_load_cohort.sql

# 2. storm it
cd tests/load/storm
go run . -base https://localhost:8443 -n 5000 -c 500 -scenario progress            # client gives up on 429
go run . -base https://localhost:8443 -n 5000 -c 500 -scenario progress -retries 6 # client backs off
go run . -base https://localhost:8443 -n 5000 -c 500 -scenario health              # the floor

# 3. put the database back
docker exec -i infra-postgres-1 psql -U qaat -d qaat -f - < tests/load/teardown_load_cohort.sql
```

Everything seeded is prefixed `LOAD-`, so the teardown removes exactly it. Seed as the owner role
(`qaat`), never `qaat_app` — RLS would hide the rows from the seeder itself.

`storm` reports the **status distribution** as well as latency. On a rate-limited endpoint that is
the whole answer: latency percentiles describe only the requests that were served, and say nothing
about the students who were turned away.

### What it found

Every student on campus Wi-Fi leaves through one public IP, and `chi.RealIP` resolves them all to
it, so a per-IP limit is in practice a per-campus limit. With `/api/v1/student/progress` at
`PublicIPRateLimit(10, 40)`, **4932 of 5000 students were refused** — 98.6%. The gateway was not
struggling; it answered the 68 it admitted with a p50 of 294ms and never returned a 5xx. It was
simply refusing almost everybody, and the hour attendance eligibility is published is exactly when
a cohort looks at once. The app-login route had already been fixed for this; progress had not.

After raising it to `(150, 1500)` — sustained rate set from measured capacity, which is ~338 req/s
for this endpoint — and teaching the clients to honour `Retry-After`: **5000 of 5000 served**, p50
954ms, p95 8.6s, no 5xx.

A client that ignores `Retry-After` still only gets 44% of them through in one shot, so the
backoff in `ProgressClient.kt` and `student-portal/src/App.tsx` is load-bearing, not a nicety.

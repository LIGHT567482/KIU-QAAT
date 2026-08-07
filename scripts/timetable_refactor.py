#!/usr/bin/env python3
"""Turn the institution's published weekend timetable into a file QAAT can import.

    python3 scripts/timetable_refactor.py ~/Downloads/data.csv -o ~/Downloads/qaat-import
    # fill in the two worksheets it writes, then run the SAME command again

WHAT IT IS FOR. The timetable the university publishes is a human document: one row per lecture,
the lecture's code and title crammed into one cell, the times as a printed range, and the college
written only on the first row of each group. QAAT's importer wants columns. Nothing about that gap
requires a person to retype 1,286 rows, so this does the mechanical part and asks for only the
facts the file genuinely does not contain.

THE TWO-PASS WORKFLOW is the whole point of the design. Two facts cannot be derived from the
published timetable — which PROGRAMME each unit belongs to, and each lecturer's STAFF ID — and
they are what the whole system hangs off: a slot with no programme has no cohort to take
attendance from, and a lecturer with no staff id cannot be matched to an account, a patrol tick or
a presence claim.

But they do not vary per row. 1,286 lectures are taught by 389 people and cover 1,174 units, so
this writes two small worksheets keyed on those instead:

    pass 1   reads the timetable  ->  writes lecturers.csv, units.csv and a draft timetable.csv
             (you fill staff_id in one, course_id in the other)
    pass 2   reads the timetable AND your filled worksheets  ->  writes the final timetable.csv

Re-running is always safe. Your edits are never overwritten: an existing worksheet is read, merged
with anything new in the source, and rewritten with your values kept.

NOTHING IS SILENTLY CORRECTED. Every change and every doubt lands in problems.csv with the source
line number, because a timetable that has been quietly "fixed" is worse than one that is visibly
wrong — the errors here decide whether a lecturer is recorded as having taught.
"""

from __future__ import annotations

import argparse
import collections
import csv
import hashlib
import os
import re
import sys

# The importer's column set, in the order it publishes as a template
# (backend/api-gateway/internal/handlers/timetable_slots.go).
TEMPLATE = ["course_id", "level", "study_year", "semester", "session_type",
            "unit_id", "unit_name", "day", "start_time", "duration_minutes", "room", "staff_id"]

SOURCE_COLS = ["College/School", "Lecturer Name", "Mobile Number", "Time", "Day",
               "Course Unit Number & Name", "Room No."]


# ─── Parsing helpers ─────────────────────────────────────────────────────────

def parse_clock(s: str):
    """'8:00 am' -> minutes since midnight, or None. Mirrors the importer's own parseClock."""
    s = s.strip().lower()
    if not s:
        return None
    pm, am = "pm" in s, "am" in s
    s = s.replace("am", "").replace("pm", "").strip()
    m = re.match(r"^(\d{1,2})(?::(\d{2}))?$", s)
    if not m:
        return None
    hh, mm = int(m.group(1)), int(m.group(2) or 0)
    if pm and hh < 12:
        hh += 12
    if am and hh == 12:
        hh = 0
    if not (0 <= hh <= 23 and 0 <= mm <= 59):
        return None
    return hh * 60 + mm


def hhmm(mins: int) -> str:
    return f"{mins // 60:02d}:{mins % 60:02d}"


def split_time_range(raw: str):
    """'10:00 am-12:00 am' -> (start, duration, note).

    THE NOON BUG, handled here because it is 266 of the 1,286 rows and it fails SILENTLY
    everywhere else. '12:00 am' is midnight, so the range ends before it starts; the importer's
    duration comes back 0 and it substitutes 60 minutes, turning a two-hour lecture into a
    one-hour one with nothing on screen to say so. An end that lands at or before the start is
    reinterpreted as the same clock time twelve hours later, which is what was meant.
    """
    raw = raw.strip()
    parts = re.split(r"\s*[-–—]\s*", raw)
    if len(parts) != 2:
        return None, None, f"time is not a range: {raw!r}"
    start, end = parse_clock(parts[0]), parse_clock(parts[1])
    if start is None or end is None:
        return None, None, f"could not read the time: {raw!r}"
    note = ""
    if end <= start:
        if end + 12 * 60 > start:
            note = (f"end time {parts[1].strip()!r} is before the start; read as "
                    f"{hhmm(end + 12 * 60)} (a 12-hour slip, almost always 'am' written for noon)")
            end += 12 * 60
        else:
            return None, None, f"end is not after the start and cannot be repaired: {raw!r}"
    return hhmm(start), end - start, note


CODE_RE = re.compile(r"^[A-Z]{2,6}[0-9]{3,5}[A-Z]?$")
UNIT_NOSEP_RE = re.compile(r"^\s*([A-Za-z]{2,6}\s?[0-9][0-9A-Za-z/]{2,14})\s+(.+)$")


def expand_codes(head: str) -> list:
    """Turn the code part of a cell into the unit ids it actually names.

    One lecture is routinely shared by several programmes and the published timetable writes that
    two ways:

        'HRM1201/DBA1203'                 two whole codes
        'ENL/LIN9113'                     two PREFIXES over one number — ENL9113 and LIN9113
        'DVS2205/PAD2203/DDS2203'         three

    All of them have to come out as separate unit ids, because in QAAT each belongs to a different
    programme with its own cohort and its own attendance. Treating the cell as one opaque code
    would file the whole lecture under a unit id no course actually has.
    """
    parts = [re.sub(r"\s+", "", p) for p in head.split("/") if p.strip()]
    tail = ""
    for p in parts:  # the number a bare prefix borrows, e.g. the 9113 in ENL/LIN9113
        m = re.search(r"([0-9]{3,5}[A-Za-z]?)$", p)
        if m:
            tail = m.group(1)
            break
    out = []
    for p in parts:
        c = (p if re.search(r"\d", p) else p + tail).upper()
        if CODE_RE.match(c) and c not in out:
            out.append(c)
    return out


def split_unit(cell: str):
    """'FIN7306 - Financial Statement Analysis' -> (['FIN7306'], 'Financial Statement Analysis')."""
    cell = cell.strip()
    # The first dash that has a code-looking head in front of it. Non-greedy so a name containing
    # a dash ('Diplomatic History of Europe 1789-1945') splits at the code, not inside the title.
    m = re.match(r"^(.{2,60}?)\s*[-–—]\s*(.+)$", cell)
    if m and re.search(r"\d", m.group(1)):
        head, name = m.group(1).strip(), m.group(2).strip()
    else:
        m = UNIT_NOSEP_RE.match(cell)  # 'DEM2101 Ecological Restoration' — no separator at all
        if not m:
            return [], cell
        head, name = m.group(1), m.group(2).strip()
    return expand_codes(head), name


def derive_year_semester(code: str):
    """KIU undergraduate codes read [PREFIX][year][semester][seq] — ACC2202 is year 2, semester 2.

    Applied ONLY to codes whose first digit is 1-3. It does not hold for the postgraduate 7/8/9
    series, where ECP7101/7201/7301/7401/7501 are module groups rather than semesters, so guessing
    there would invent cohorts that do not exist. Those come back blank for a human to fill.
    """
    m = re.search(r"([0-9])([0-9])[0-9]{1,2}$", code)
    if not m:
        return "", ""
    year, sem = int(m.group(1)), int(m.group(2))
    if 1 <= year <= 3 and 1 <= sem <= 2:
        return str(year), str(sem)
    return "", ""


def room_key(s: str) -> str:
    """Group spellings of one room. Keeps the discriminating parts and drops only the noise.

    'D05 O.B' and 'D05 O.B.' are one room; 'MAIN HALL(A)' and 'MAIN HALL (B)' are two, so the
    bracketed letter has to survive normalisation while the trailing full stop does not.
    """
    s = s.upper().strip().rstrip(".")
    s = re.sub(r"\s*\(\s*", " (", s)
    s = re.sub(r"\s*\)\s*", ") ", s)
    return re.sub(r"\s+", " ", s).strip()


# The courtesy titles the published timetable actually uses, longest first so "Assoc.Prof." is
# matched before the "Prof." inside it.
TITLE_RE = re.compile(
    r"^(assoc\.?\s*prof\.?|asst\.?\s*prof\.?|prof\.?|dr\.?|mrs\.?|mr\.?|ms\.?|miss|eng\.?|rev\.?|sr\.?|hon\.?)"
    r"\s*\.?\s+", re.I)

# One spelling per title. The file writes 'Mr.', 'MR.' and 'Mr' for the same thing, and the
# dashboards render title + name as one string, so three spellings become three different-looking
# people on the same screen.
TITLE_CANON = {"dr": "Dr.", "mr": "Mr.", "mrs": "Mrs.", "ms": "Ms.", "miss": "Miss",
               "prof": "Prof.", "assocprof": "Assoc. Prof.", "asstprof": "Asst. Prof.",
               "eng": "Eng.", "rev": "Rev.", "sr": "Sr.", "hon": "Hon."}


# 'PendingO', 'Pending_SEAS_2', 'Not Set 23' — the timetable office's marker for a lecture with
# nobody assigned yet. Spelled at least eight ways, so matching only 'Pending' missed sixteen of
# the seventeen and let them through as if they were staff.
PLACEHOLDER_RE = re.compile(r"^\s*(pending|not\s*set|tba|tbd)", re.I)
# No \b after the group: the file writes 'PendingO', 'Pending11' and 'Pending_SEAS' with no
# boundary between the word and what follows, so a \b matched two of the sixteen. And no 'n/a'
# in the alternation — unanchored it would swallow every name beginning "Na".


def is_placeholder(name: str) -> bool:
    return bool(PLACEHOLDER_RE.match(name or ""))


def split_title(published: str):
    """'Assoc.Prof. Arthur Sunday' -> ('Assoc. Prof.', 'Arthur Sunday').

    The lecturers table holds title and full_name in separate columns and every dashboard joins
    them back together for display, so leaving the title inside the name would print it twice.
    A name with no title keeps a blank one — 17 of the 389 have none, and inventing one would be
    asserting something about a person that the source never said.
    """
    name = published.strip()
    found = []
    # A LOOP, not one match: 'Assoc.Prof. Dr. Kareyo Margaret' carries two titles — an academic
    # rank and a doctorate — and stripping only the first leaves 'Dr.' inside the name, where
    # every dashboard would print it after the title it already shows.
    while True:
        m = TITLE_RE.match(name)
        if not m:
            break
        key = re.sub(r"[^a-z]", "", m.group(1).lower())
        found.append(TITLE_CANON.get(key, m.group(1).strip()))
        name = name[m.end():].strip()
    return " ".join(found), name


STAFF_ID_RE = re.compile(r"^KIU-\d{5}$")


def make_staff_id(full_name: str, taken: set) -> str:
    """A stand-in staff id, KIU-XXXXX, for an institution that has not supplied its real ones.

    DERIVED FROM THE NAME, NOT DRAWN AT RANDOM, and the difference matters the moment this is used
    twice. These ids go into the database as the key that ties a lecturer to their account, their
    patrol ticks and their presence claims. If the output folder were deleted and the script re-run
    — which is the ordinary way to pick up a corrected timetable — freshly random ids would give
    every lecturer a second identity, and the second import would create 389 duplicate people
    rather than update the existing ones. Hashing the name means the same person always lands on
    the same id, on any machine, forever.

    It still looks arbitrary, because it is meant to be a placeholder rather than a meaningful
    code: nothing should ever be inferred from the digits.

    Collisions are probed forward. 389 names over 90,000 slots makes one likely by the birthday
    bound (~57% chance), so this is not a theoretical branch.
    """
    h = int(hashlib.sha256(full_name.strip().lower().encode()).hexdigest(), 16)
    n = h % 90000 + 10000  # 10000-99999, so it is always exactly five digits
    for i in range(90000):
        sid = f"KIU-{(n - 10000 + i) % 90000 + 10000:05d}"
        if sid not in taken:
            return sid
    raise RuntimeError("ran out of staff ids")  # unreachable at this scale


def normalise_phone(s: str) -> str:
    """Ugandan mobile numbers, written five different ways in this file, to a single 07XXXXXXXX."""
    d = re.sub(r"\D", "", s or "")
    if not d:
        return ""
    if d.startswith("256") and len(d) == 12:
        d = "0" + d[3:]
    elif len(d) == 9:
        d = "0" + d
    return d if re.fullmatch(r"0\d{9}", d) else s.strip()


# ─── Worksheets ──────────────────────────────────────────────────────────────

def read_worksheet(path: str, key: str) -> dict:
    """Read a worksheet the user may have filled in. Missing file = empty, not an error."""
    if not os.path.exists(path):
        return {}
    with open(path, newline="", encoding="utf-8-sig") as fh:
        return {r[key].strip(): r for r in csv.DictReader(fh) if r.get(key, "").strip()}


def write_worksheet(path: str, cols: list, rows: list):
    with open(path, "w", newline="", encoding="utf-8") as fh:
        w = csv.DictWriter(fh, fieldnames=cols)
        w.writeheader()
        w.writerows(rows)




README = """# KIU weekend timetable -> QAAT import

Generated by `scripts/timetable_refactor.py` from `{source}`. Re-run it any time; it never
overwrites what you type in these files.

## The four files

| File | Rows | What it is |
|---|---|---|
| **timetable.csv** | {slots} | The importer's own column order. **This is what you upload.** |
| **lecturers.csv** | {lecturers} | The `lecturers` table's own columns, in its order, so this doubles as the lecturer-account import. `staff_id` is already filled. |
| **units.csv** | {units} | Fill **course_id** — the programme each unit belongs to. |
| **problems.csv** | {problems} | Everything worth a human eye, each with its line number in the original. |

## What to do

1. **units.csv** -> fill `course_id`. Sorted by code with the college beside it, so whole blocks
   share a programme: fill one, drag down. `level`, `study_year` and `semester` are pre-filled for
   undergraduate codes; add them for the 7000/8000/9000 postgraduate ones, where the code does not
   encode a year.
2. **lecturers.csv** -> nothing is required. Overwrite any generated `staff_id` with the real
   number whenever you have it; it will never be regenerated. `email` and `gender` are yours to add.
3. Run the script again, exactly as before. It reads both files back and writes the finished
   `timetable.csv`, and says plainly what is still blank.
4. Upload `timetable.csv` at **Admin -> Timetable -> Import**.

Nothing imports until every slot has a `course_id`: a lecture with no programme has no cohort, and
a cohort is what attendance is taken from. A blank `staff_id` is NOT a blocker — that slot simply
imports with no lecturer attached.

## What was done to the data

- **College carried down** on {college_blank} rows — the published file writes it once per group
  (a merged cell), so the blanks are continuations, not gaps.
- **{time_repaired} end times repaired.** `10:00 am-12:00 am` means noon, not midnight. Left alone
  it is worse than an error: the importer reads the range as ending before it starts, quietly falls
  back to 60 minutes, and a two-hour lecture is filed as one hour with nothing on screen to say so.
- **{added} slots added** where one cell named several unit codes (`HRM1201/DBA1203`,
  `ENL/LIN9113`, `DVS2205/PAD2203/DDS2203`). Each programme's cohort attends that lecture, so each
  needs its own row. Delete any that do not apply.
- **session_type = Weekend** on every row. Every lecture here is a Saturday or Sunday, and the
  importer refuses a Day cohort on a weekend — without this, all {slots} would be skipped.
- **Titles separated from names.** `Assoc.Prof. Arthur Sunday` -> title `Assoc. Prof.`, name
  `Arthur Sunday`, because the lecturers table holds them apart and every dashboard rejoins them.
  Spellings folded to one form each (`MR.`, `Mr` -> `Mr.`); doubled titles kept whole
  (`Assoc. Prof. Dr.`).
- **{staff_made} staff ids generated** — `KIU-XXXXX`, five digits. DERIVED FROM THE NAME rather
  than drawn at random, so deleting this folder and re-running gives every lecturer the same id
  again. Random ones would hand each person a second identity on the next run, and the next import
  would create duplicate lecturers instead of updating the existing ones. Read nothing into the
  digits. A staff id you type in yourself is never overwritten. The {placeholders} placeholder rows
  (`PendingO`, `Not Set 23`...) deliberately get none — issuing them an id would create lecturers
  who do not exist. Use `--no-staff-ids` to skip this entirely.
- Times split into `start_time` + `duration_minutes`; unit cells split into `unit_id` +
  `unit_name`; phones normalised to `07XXXXXXXX`.

## What is left for a person

Read `problems.csv`:

{problem_list}

Room spellings need no work: all {rooms} are used consistently across the file.
"""


# ─── Main ────────────────────────────────────────────────────────────────────

def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("source", help="the published timetable CSV")
    ap.add_argument("-o", "--out", default="qaat-import", help="output directory")
    ap.add_argument("--no-staff-ids", action="store_true",
                    help="do not invent KIU-XXXXX placeholders for lecturers with no staff id")
    args = ap.parse_args()
    os.makedirs(args.out, exist_ok=True)

    with open(args.source, newline="", encoding="utf-8-sig") as fh:
        src = list(csv.DictReader(fh))
    missing = [c for c in SOURCE_COLS if c not in (src[0] if src else {})]
    if missing:
        print(f"error: the source is missing {missing}", file=sys.stderr)
        return 1

    lect_path, unit_path = os.path.join(args.out, "lecturers.csv"), os.path.join(args.out, "units.csv")
    known_lect, known_unit = read_worksheet(lect_path, "full_name"), read_worksheet(unit_path, "unit_id")
    second_pass = bool(known_lect or known_unit)

    problems, out_rows = [], []
    lecturers: dict[str, dict] = {}
    units: dict[str, dict] = {}
    unit_names = collections.defaultdict(collections.Counter)
    room_spellings = collections.defaultdict(collections.Counter)

    def flag(line, kind, detail, action=""):
        problems.append({"source_line": line, "issue": kind, "detail": detail, "what_was_done": action})

    # Pass A — read, repair, and collect the vocabularies.
    college = ""
    for i, r in enumerate(src, start=2):  # 2 = first data row, matching a spreadsheet's numbering
        # The college is written once per group in the published document (a merged cell), so 295
        # rows arrive blank. Carrying it down is what the printed table means, not a guess.
        c = r["College/School"].strip()
        if c:
            college = c
        elif college:
            flag(i, "college blank", "carried down from the row above (merged cell in the source)")
        r = dict(r, **{"College/School": college})

        start, dur, note = split_time_range(r["Time"])
        if start is None:
            flag(i, "unusable time", note, "ROW DROPPED")
            continue
        if note:
            flag(i, "time repaired", note, f"start {start}, {dur} min")
        if dur != 120:
            flag(i, "unusual length", f"{r['Time'].strip()!r} is {dur} minutes; almost every other "
                                      f"lecture is 120 — check this is what was meant")

        codes, name = split_unit(r["Course Unit Number & Name"])
        if not codes:
            flag(i, "no unit code", f"could not find a code in {r['Course Unit Number & Name']!r}", "ROW DROPPED")
            continue
        if len(codes) > 1:
            flag(i, "one lecture, several unit codes", f"{r['Course Unit Number & Name'].strip()!r}",
                 f"emitted as {len(codes)} slots ({', '.join(codes)}) — each programme's cohort "
                 f"attends it, so each needs its own row. Delete any that do not apply.")

        room = r["Room No."].strip()
        room_spellings[room_key(room)][room] += 1
        if ":" in room or len(room) > 22:
            flag(i, "room looks contaminated", f"{room!r} — unit text appears to have bled into this column")

        lect = r["Lecturer Name"].strip()
        if is_placeholder(lect):
            flag(i, "no lecturer", f"{lect!r} is a placeholder, not a person",
                 "slot kept, staff_id left blank — assign someone before the term starts")
        title, full_name = split_title(lect)
        lecturers.setdefault(lect, {"staff_id": "", "full_name": full_name, "email": "",
                                    "phone": normalise_phone(r["Mobile Number"]),
                                    "title": title, "gender": ""})

        for code in codes:
            unit_names[code][name] += 1
            y, s = derive_year_semester(code)
            units.setdefault(code, {"unit_id": code, "unit_name": name, "college": college,
                                    "course_id": "", "level": "", "study_year": y, "semester": s})
            out_rows.append({"_line": i, "college": college, "lecturer": lect, "unit_id": code,
                             "unit_name": name, "day": r["Day"].strip(), "start_time": start,
                             "duration_minutes": dur, "room": room})

    # A code that names two different units. course_units.unit_id is the primary key and the
    # importer does ON CONFLICT DO NOTHING, so the first name wins and the rest vanish — the
    # collision has to be settled here or it is settled by row order.
    for code, names in unit_names.items():
        if len(names) > 1:
            chosen, _ = names.most_common(1)[0]
            others = ", ".join(f"{n!r}" for n, _ in names.most_common()[1:])
            flag("", "one code, several names", f"{code}: {chosen!r} vs {others}",
                 f"used {chosen!r} — confirm these are the same unit")
            units[code]["unit_name"] = chosen

    # Canonical room spelling: the most common one wins, and every rewrite is listed.
    canon = {}
    for key, spellings in room_spellings.items():
        best, _ = spellings.most_common(1)[0]
        canon[key] = best
        if len(spellings) > 1:
            flag("", "room spelled several ways", f"{', '.join(repr(s) for s in spellings)}",
                 f"all written as {best!r}")

    # Two lectures, one lecturer, one hour. Directly contradicts the rule the system enforces —
    # a lecturer handles one session at a time — so it cannot be left for the phone to discover.
    by_lect = collections.defaultdict(list)
    for row in out_rows:
        by_lect[(row["lecturer"], row["day"], row["start_time"])].append(row)
    for (lect, day, start), group in sorted(by_lect.items()):
        # Rows expanded from ONE cell are one lecture, not a clash with itself.
        if len({g["_line"] for g in group}) > 1 and not is_placeholder(lect):
            where = "; ".join(f"{g['unit_id']} in {g['room']}" for g in group)
            flag(", ".join(str(g["_line"]) for g in group), "lecturer double-booked",
                 f"{lect} — {day} {start}: {where}", "both kept; the timetable office must resolve it")

    # Same room, same hour, two lectures.
    by_room = collections.defaultdict(list)
    for row in out_rows:
        by_room[(canon[room_key(row["room"])], row["day"], row["start_time"])].append(row)
    for (room, day, start), group in sorted(by_room.items()):
        if len({g["_line"] for g in group}) > 1:
            flag(", ".join(str(g["_line"]) for g in group), "room double-booked",
                 f"{room} — {day} {start}: " + "; ".join(f"{g['unit_id']} ({g['lecturer']})" for g in group),
                 "both kept")

    # Pass B — merge the worksheets and write the import file.
    for _, row in lecturers.items():
        prev = known_lect.get(row["full_name"])
        if prev:
            # Every column is carried back, not just staff_id: a corrected phone, a supplied
            # gender or a fixed-up title are edits too, and silently discarding them would
            # teach the operator that re-running loses work.
            for f in ("staff_id", "email", "phone", "title", "gender"):
                if prev.get(f, "").strip():
                    row[f] = prev[f].strip()
    for code, row in units.items():
        if code in known_unit:
            for f in ("course_id", "level", "study_year", "semester"):
                if known_unit[code].get(f, "").strip():
                    row[f] = known_unit[code][f].strip()

    # EXACTLY the column set and order of the lecturers table, so this file is also the
    # lecturer-account import — one file, not two, and no re-typing of 389 names.
    if not args.no_staff_ids:
        # Only ever FILLS BLANKS. A staff id already in the sheet is the institution's own and is
        # never touched — the generated ones are scaffolding, to be replaced as the real numbers
        # arrive, one row at a time, without disturbing anything else.
        taken = {r["staff_id"].strip() for r in lecturers.values() if r["staff_id"].strip()}
        made = 0
        for row in sorted(lecturers.values(), key=lambda r: r["full_name"]):
            if row["staff_id"].strip():
                continue
            if is_placeholder(row["full_name"]):
                # 'PendingO' and 'Not Set 23' are the timetable office's marker for a lecture with
                # nobody assigned. Issuing them a staff id would create sixteen lecturers who do
                # not exist, and they would be indistinguishable from real staff thereafter.
                continue
            row["staff_id"] = make_staff_id(row["full_name"], taken)
            taken.add(row["staff_id"])
            made += 1
        if made:
            flag("", "staff ids generated", f"{made} lecturers had none",
                 "KIU-XXXXX derived from the name, so re-running is stable; replace as the real numbers arrive")

    write_worksheet(lect_path, ["staff_id", "full_name", "email", "phone", "title", "gender"],
                    sorted(lecturers.values(), key=lambda r: r["full_name"]))
    write_worksheet(unit_path, ["unit_id", "unit_name", "college", "course_id", "level", "study_year", "semester"],
                    sorted(units.values(), key=lambda r: r["unit_id"]))

    final = []
    for row in out_rows:
        u, l = units[row["unit_id"]], lecturers[row["lecturer"]]
        day = row["day"]
        final.append({
            "course_id": u["course_id"],
            "level": u["level"],
            "study_year": u["study_year"],
            "semester": u["semester"],
            # Every row in this file is a Saturday or a Sunday. The importer refuses a Day cohort
            # on a weekend, so without this EVERY row would be skipped with a message about the
            # cohort rather than the missing column.
            "session_type": "Weekend" if day.lower().startswith(("sat", "sun")) else "Day",
            "unit_id": row["unit_id"],
            "unit_name": u["unit_name"],
            "day": day,
            "start_time": row["start_time"],
            "duration_minutes": row["duration_minutes"],
            "room": canon[room_key(row["room"])],
            "staff_id": l["staff_id"],
        })

    dropped_rows = sum(1 for x in problems if "DROPPED" in x["what_was_done"])
    tt_path = os.path.join(args.out, "timetable.csv")
    write_worksheet(tt_path, TEMPLATE, final)
    write_worksheet(os.path.join(args.out, "problems.csv"),
                    ["source_line", "issue", "detail", "what_was_done"], problems)

    # The README is GENERATED, not hand-written beside the output. A hand-written one is lost the
    # first time someone clears the folder and re-runs — which is the ordinary way to pick up a
    # corrected timetable — and the numbers in it drift from the data the moment anything changes.
    counts = collections.Counter(x["issue"] for x in problems)
    listed = "\n".join(f"- **{n} {k}**" for k, n in counts.most_common()
                        if k not in ("college blank", "time repaired", "staff ids generated"))
    with open(os.path.join(args.out, "README.md"), "w", encoding="utf-8") as fh:
        fh.write(README.format(
            source=args.source, slots=len(final), lecturers=len(lecturers), units=len(units),
            problems=len(problems), college_blank=counts["college blank"],
            time_repaired=counts["time repaired"], added=len(final) - (len(src) - dropped_rows),
            staff_made=sum(1 for r in lecturers.values() if STAFF_ID_RE.match(r["staff_id"])),
            placeholders=sum(1 for r in lecturers.values() if is_placeholder(r["full_name"])),
            rooms=len(canon), problem_list=listed or "- nothing outstanding"))

    dropped = dropped_rows
    no_course = sum(1 for r in final if not r["course_id"])
    no_staff = sum(1 for r in final if not r["staff_id"])
    print(f"read      {len(src)} rows from {args.source}")
    # Slots can EXCEED rows: a cell naming several unit codes is several lectures, one per cohort.
    print(f"wrote     {len(final)} slots   ({dropped} rows dropped, "
          f"{len(final) - (len(src) - dropped)} added by splitting shared-code cells)")
    print(f"lecturers {len(lecturers)}   units {len(units)}   rooms {len(canon)}")
    print(f"problems  {len(problems)} noted")
    print()
    print(f"  {tt_path}")
    print(f"  {lect_path}")
    print(f"  {unit_path}")
    print(f"  {os.path.join(args.out, 'problems.csv')}")
    print()
    # course_id is FATAL and staff_id is not, and saying so matters: the importer needs a course
    # to resolve the offering a slot belongs to, but it treats staff_id as optional and simply
    # leaves the slot unlinked. Reporting both as blockers would send someone hunting for staff
    # numbers they do not need before they can import at all.
    if no_course:
        print(f"NOT READY: {no_course} of {len(final)} slots have no course_id.")
        print("  Fill the course_id column in units.csv, then run this exact command again —")
        print("  your entries are read back and kept.")
    else:
        print(f"READY TO IMPORT: every one of the {len(final)} slots has a course_id.")
    if no_staff:
        print(f"\n{no_staff} slots have no staff_id. Not a blocker — those slots import with no")
        print("  lecturer attached, which is right for the placeholder rows and wrong for anyone else.")
    if not second_pass:
        print("\nRead problems.csv before importing.")
    return 0


if __name__ == "__main__":
    sys.exit(main())

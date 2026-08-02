#!/usr/bin/env python3
"""
QAAT Demo Setup — Full University Architecture
Clears all data and seeds a realistic multi-coordinator, multi-intake,
multi-year university dataset. Run once against a clean or dirty DB.

Architecture modelled:
  • One tenant: Nairobi University of Technology
  • Three courses, each with its OWN coordinator
  • Course units spread across Year 1 / 2 / 3, Semester 1 & 2
  • Students in different academic years (Year 1 = 2025/2026,
    Year 2 = 2024/2025, Year 3 = 2023/2024) with different intake
    session types (Morning, Evening, Weekend, Distance)
  • Today's sessions cover all four session modes across the three courses
  • Admin dashboard is the canonical entry-point for creating staff accounts
"""

import hashlib, os, base64, sys
import psycopg2
from datetime import date

# ── deps check ────────────────────────────────────────────────────────────────
for pkg in ("bcrypt", "pyotp", "cryptography"):
    try:
        __import__(pkg.split(".")[0])
    except ImportError:
        os.system(f"pip install {pkg} -q")

import bcrypt
import pyotp
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

# ── helpers ───────────────────────────────────────────────────────────────────

def phash(pw: str) -> str:
    return bcrypt.hashpw(pw.encode(), bcrypt.gensalt(rounds=12)).decode()

def derive_key(passphrase: str) -> bytes:
    return hashlib.sha256((passphrase + "-qaat-totp-v1").encode()).digest()

def encrypt_secret(plaintext: str, passphrase: str) -> str:
    key   = derive_key(passphrase)
    nonce = os.urandom(12)
    ct    = AESGCM(key).encrypt(nonce, plaintext.encode(), None)
    return base64.b64encode(nonce + ct).decode()

def gen_totp(email: str, pw_hash: str):
    secret = pyotp.random_base32()
    url    = pyotp.totp.TOTP(secret).provisioning_uri(name=email, issuer_name="QAAT")
    enc    = encrypt_secret(secret, pw_hash)
    return secret, url, enc

TODAY = date.today().isoformat()

# ── passwords ─────────────────────────────────────────────────────────────────
PASSWORDS = {
    "admin":    "Admin@QAAT1",
    "vc":       "VC@QAAT1",
    "dqa":      "DQA@QAAT1",
    "qa":       "QA@QAAT1",
    "coord_cs": "CoordCS@1",
    "coord_se": "CoordSE@1",
    "coord_it": "CoordIT@1",
    "student":  "Student@QAAT1",
}

print("Hashing passwords (bcrypt cost 12, ~10 s for 8 hashes)…")
hashes = {k: phash(v) for k, v in PASSWORDS.items()}

# ── connect ───────────────────────────────────────────────────────────────────
conn = psycopg2.connect(host="127.0.0.1", port=5434, dbname="qaat",
                        user="qaat", password="qaat")
conn.autocommit = False
cur = conn.cursor()

# ── wipe everything ───────────────────────────────────────────────────────────
print("Cleaning all data…")
cur.execute("""
TRUNCATE
  attendance_logs,
  hardware_vault,
  student_attendance_summary,
  sessions,
  sync_uploads,
  students_extended,
  course_units,
  courses,
  venues,
  lecturer_attendance_logs,
  admin_audit_log,
  users
RESTART IDENTITY CASCADE;
DELETE FROM tenants;
""")

# ── tenant ────────────────────────────────────────────────────────────────────
print("Creating tenant…")
cur.execute("""
INSERT INTO tenants (name, domain, rsa_key_id, attendance_threshold,
                     checkin_window_minutes, auto_kill_minutes)
VALUES ('Nairobi University of Technology', 'nut.ac.ke', 'pending', 75, 30, 180)
RETURNING tenant_id, student_hash_key
""")
tenant_id, student_hash_key = cur.fetchone()
print(f"  tenant_id: {tenant_id}")

# ── staff users ───────────────────────────────────────────────────────────────
print("Creating staff users…")

def add_user(email, role, pw_key, full_name):
    cur.execute("""
        INSERT INTO users (tenant_id, email, password_hash, role, full_name)
        VALUES (%s,%s,%s,%s,%s) RETURNING user_id
    """, (tenant_id, email, hashes[pw_key], role, full_name))
    return cur.fetchone()[0]

admin_id   = add_user("admin@nut.ac.ke",       "ADMIN",        "admin",    "Platform Admin")
vc_id      = add_user("vc@nut.ac.ke",          "VC",           "vc",       "Prof. Alice Kamau")
dqa_id     = add_user("dqa@nut.ac.ke",         "DQA_DIRECTOR", "dqa",      "Dr. Brian Otieno")
qa_id      = add_user("qa@nut.ac.ke",          "QA_OFFICER",   "qa",       "Ms. Carol Njeri")

# Three coordinators — one per course (admin creates these in real deployments)
coord_cs_id = add_user("coord.cs@nut.ac.ke",  "COORDINATOR",  "coord_cs", "Dr. James Mwangi")
coord_se_id = add_user("coord.se@nut.ac.ke",  "COORDINATOR",  "coord_se", "Dr. Sarah Odhiambo")
coord_it_id = add_user("coord.it@nut.ac.ke",  "COORDINATOR",  "coord_it", "Mr. Peter Kamau")

# ── TOTP for VC and DQA ───────────────────────────────────────────────────────
print("Setting up TOTP for VC and DQA…")

vc_totp_secret,  vc_url,  vc_enc  = gen_totp("vc@nut.ac.ke",  hashes["vc"])
dqa_totp_secret, dqa_url, dqa_enc = gen_totp("dqa@nut.ac.ke", hashes["dqa"])

vc_backup  = encrypt_secret("unused", hashes["vc"])
dqa_backup = encrypt_secret("unused", hashes["dqa"])

cur.execute("""
    UPDATE users SET totp_secret_enc=%s, totp_enabled=true, totp_backup_codes_enc=%s
    WHERE user_id=%s
""", (vc_enc, vc_backup, vc_id))
cur.execute("""
    UPDATE users SET totp_secret_enc=%s, totp_enabled=true, totp_backup_codes_enc=%s
    WHERE user_id=%s
""", (dqa_enc, dqa_backup, dqa_id))

# ── venues ────────────────────────────────────────────────────────────────────
print("Creating venues…")
venues = [
    ("MLH-01", "Main Lecture Hall"),
    ("LAB-A1", "Science Lab Block A"),
    ("SEM-01", "Seminar Room 01"),
    ("AUD-01", "Main Auditorium"),
    ("ICT-01", "ICT Centre Room 1"),
]
for vid, vname in venues:
    cur.execute("INSERT INTO venues (venue_id, tenant_id, name) VALUES (%s,%s,%s)",
                (vid, tenant_id, vname))

# ── courses (one coordinator each) ────────────────────────────────────────────
print("Creating courses…")
cur.execute("""
    INSERT INTO courses (course_id, tenant_id, name, coordinator_id, department)
    VALUES
      ('CS-DEPT', %s, 'BSc Computer Science',       %s, 'School of Computing'),
      ('SE-DEPT', %s, 'BSc Software Engineering',   %s, 'School of Computing'),
      ('IT-DEPT', %s, 'BSc Information Technology', %s, 'School of IT')
""", (tenant_id, coord_cs_id,
      tenant_id, coord_se_id,
      tenant_id, coord_it_id))

# ── course units: three years × two semesters per course ─────────────────────
print("Creating course units…")
# (unit_id, course_id, name, year, semester, academic_year)
units = [
    # ── BSc Computer Science ──────────────────────────────────────────────────
    ("CS101","CS-DEPT","Introduction to Programming",           1, 1, "2025/2026"),
    ("CS102","CS-DEPT","Discrete Mathematics",                  1, 2, "2025/2026"),
    ("CS201","CS-DEPT","Data Structures & Algorithms",          2, 1, "2024/2025"),
    ("CS202","CS-DEPT","Operating Systems",                     2, 2, "2024/2025"),
    ("CS301","CS-DEPT","Database Systems",                      3, 1, "2023/2024"),
    ("CS302","CS-DEPT","Computer Networks",                     3, 2, "2023/2024"),

    # ── BSc Software Engineering ──────────────────────────────────────────────
    ("SE101","SE-DEPT","Software Engineering Fundamentals",     1, 1, "2025/2026"),
    ("SE102","SE-DEPT","Requirements Engineering",              1, 2, "2025/2026"),
    ("SE201","SE-DEPT","Software Design & Architecture",        2, 1, "2024/2025"),
    ("SE202","SE-DEPT","Software Testing & QA",                 2, 2, "2024/2025"),
    ("SE301","SE-DEPT","DevOps & Continuous Delivery",          3, 1, "2023/2024"),
    ("SE302","SE-DEPT","Agile Project Management",              3, 2, "2023/2024"),

    # ── BSc Information Technology ────────────────────────────────────────────
    ("IT101","IT-DEPT","Computer Fundamentals",                 1, 1, "2025/2026"),
    ("IT102","IT-DEPT","Digital Literacy & Office Tools",       1, 2, "2025/2026"),
    ("IT201","IT-DEPT","Web Technologies",                      2, 1, "2024/2025"),
    ("IT202","IT-DEPT","Database Administration",               2, 2, "2024/2025"),
    ("IT301","IT-DEPT","Network Administration",                3, 1, "2023/2024"),
    ("IT302","IT-DEPT","Cybersecurity Fundamentals",            3, 2, "2023/2024"),
]
for uid, cid, uname, yr, sem, acyr in units:
    cur.execute("""
        INSERT INTO course_units (unit_id, tenant_id, course_id, name, year, semester, academic_year)
        VALUES (%s,%s,%s,%s,%s,%s,%s)
    """, (uid, tenant_id, cid, uname, yr, sem, acyr))

# ── students across three years and four session types ────────────────────────
print("Creating students across years, intakes, and session types…")

# (student_id, full_name, email, course_id, intake, current_year, semester, academic_year)
students = [
    # ── CS Year 3 — Morning class (2023/2024 intake) ──────────────────────────
    ("2023/CS/001","Faith Achieng",  "f.achieng@students.nut.ac.ke",  "CS-DEPT","Morning",3,2,"2023/2024"),
    ("2023/CS/002","George Maina",   "g.maina@students.nut.ac.ke",    "CS-DEPT","Morning",3,2,"2023/2024"),
    ("2023/CS/003","Hannah Wangari", "h.wangari@students.nut.ac.ke",  "CS-DEPT","Morning",3,2,"2023/2024"),

    # ── CS Year 2 — Evening class (2024/2025 intake) ──────────────────────────
    ("2024/CS/001","Isaac Odhiambo", "i.odhiambo@students.nut.ac.ke", "CS-DEPT","Evening",2,1,"2024/2025"),
    ("2024/CS/002","Joyce Njoroge",  "j.njoroge@students.nut.ac.ke",  "CS-DEPT","Evening",2,1,"2024/2025"),

    # ── CS Year 1 — Weekend class (2025/2026 intake) ──────────────────────────
    ("2025/CS/001","Kevin Mutua",    "k.mutua@students.nut.ac.ke",    "CS-DEPT","Weekend",1,1,"2025/2026"),
    ("2025/CS/002","Linda Chebet",   "l.chebet@students.nut.ac.ke",   "CS-DEPT","Weekend",1,1,"2025/2026"),

    # ── SE Year 2 — Morning class (2024/2025 intake) ──────────────────────────
    ("2024/SE/001","Moses Kariuki",  "m.kariuki@students.nut.ac.ke",  "SE-DEPT","Morning",2,1,"2024/2025"),
    ("2024/SE/002","Nancy Wanjiku",  "n.wanjiku@students.nut.ac.ke",  "SE-DEPT","Morning",2,1,"2024/2025"),

    # ── SE Year 1 — Distance/Online (2025/2026 intake) ───────────────────────
    ("2025/SE/001","Peter Omondi",   "p.omondi@students.nut.ac.ke",   "SE-DEPT","Distance",1,1,"2025/2026"),
    ("2025/SE/002","Queen Adhiambo", "q.adhiambo@students.nut.ac.ke", "SE-DEPT","Distance",1,1,"2025/2026"),

    # ── IT Year 3 — Weekend class (2023/2024 intake) ──────────────────────────
    ("2023/IT/001","Robert Ng'ang'a","r.nganga@students.nut.ac.ke",   "IT-DEPT","Weekend",3,2,"2023/2024"),
    ("2023/IT/002","Susan Wambua",   "s.wambua@students.nut.ac.ke",   "IT-DEPT","Weekend",3,2,"2023/2024"),

    # ── IT Year 1 — Evening class (2025/2026 intake) ─────────────────────────
    ("2025/IT/001","Thomas Koech",   "t.koech@students.nut.ac.ke",    "IT-DEPT","Evening",1,1,"2025/2026"),
    ("2025/IT/002","Uma Atieno",     "u.atieno@students.nut.ac.ke",   "IT-DEPT","Evening",1,1,"2025/2026"),
]

for sid, name, email, cid, intake, yr, sem, acyr in students:
    cur.execute("""
        INSERT INTO students_extended
          (student_id, tenant_id, full_name, email, course_id,
           intake_session, current_year, semester, academic_year)
        VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s)
    """, (sid, tenant_id, name, email, cid, intake, yr, sem, acyr))
    cur.execute("""
        INSERT INTO users (tenant_id, email, password_hash, role, full_name)
        VALUES (%s,%s,%s,'STUDENT',%s)
    """, (tenant_id, email, hashes["student"], name))

# ── today's sessions: four different session types across three courses ────────
print("Scheduling today's sessions…")

# Each coordinator runs their own session type today.
# (unit_id, coordinator_id, venue_id) — all for TODAY
sessions = [
    # CS coord — Year 3 Morning class, Database Systems (Sem 2)
    ("CS301", coord_cs_id, "MLH-01"),
    # CS coord — Year 2 Evening class, Data Structures (Sem 1)
    ("CS201", coord_cs_id, "AUD-01"),
    # SE coord — Year 2 Morning class, Software Design
    ("SE201", coord_se_id, "SEM-01"),
    # SE coord — Year 1 Distance/Online, SE Fundamentals
    ("SE101", coord_se_id, "ICT-01"),
    # IT coord — Year 3 Weekend class, Network Admin
    ("IT301", coord_it_id, "LAB-A1"),
    # IT coord — Year 1 Evening class, Computer Fundamentals
    ("IT101", coord_it_id, "SEM-01"),
]

for unit_id, coord_id, venue_id in sessions:
    cur.execute("""
        INSERT INTO sessions (tenant_id, coordinator_id, unit_id, venue_id, session_date, session_status)
        VALUES (%s,%s,%s,%s,%s,'PENDING_LECTURER')
    """, (tenant_id, coord_id, unit_id, venue_id, TODAY))

conn.commit()
cur.close()
conn.close()

# ── print credentials ─────────────────────────────────────────────────────────
print()
print("=" * 72)
print("  QAAT  — Nairobi University of Technology  DEMO SETUP COMPLETE")
print("=" * 72)
print(f"\n  Tenant ID: {tenant_id}")
print()
print(f"  {'ROLE':<18} {'EMAIL':<42} PASSWORD")
print(f"  {'-'*18} {'-'*42} {'-'*16}")
staff = [
    ("ADMIN",       "admin@nut.ac.ke",           PASSWORDS["admin"]),
    ("VC",          "vc@nut.ac.ke",              PASSWORDS["vc"] + " + TOTP"),
    ("DQA_DIRECTOR","dqa@nut.ac.ke",             PASSWORDS["dqa"] + " + TOTP"),
    ("QA_OFFICER",  "qa@nut.ac.ke",              PASSWORDS["qa"]),
    ("COORDINATOR", "coord.cs@nut.ac.ke",        PASSWORDS["coord_cs"]
                                                 + "  ← CS Dept (Dr. Mwangi)"),
    ("COORDINATOR", "coord.se@nut.ac.ke",        PASSWORDS["coord_se"]
                                                 + "  ← SE Dept (Dr. Odhiambo)"),
    ("COORDINATOR", "coord.it@nut.ac.ke",        PASSWORDS["coord_it"]
                                                 + "  ← IT Dept (Mr. Kamau)"),
]
for role, email, pw in staff:
    print(f"  {role:<18} {email:<42} {pw}")

print()
print("  STUDENT ACCOUNTS  (all share password: " + PASSWORDS["student"] + ")")
print(f"  {'REG NO':<16} {'NAME':<22} {'COURSE':<10} {'YEAR':<7} {'SESSION TYPE'}")
print(f"  {'-'*16} {'-'*22} {'-'*10} {'-'*7} {'-'*12}")
student_display = [
    ("2023/CS/001","Faith Achieng",  "CS", "Year 3","Morning"),
    ("2024/CS/001","Isaac Odhiambo", "CS", "Year 2","Evening"),
    ("2025/CS/001","Kevin Mutua",    "CS", "Year 1","Weekend"),
    ("2024/SE/001","Moses Kariuki",  "SE", "Year 2","Morning"),
    ("2025/SE/001","Peter Omondi",   "SE", "Year 1","Distance"),
    ("2023/IT/001","Robert Ng'ang'a","IT", "Year 3","Weekend"),
    ("2025/IT/001","Thomas Koech",   "IT", "Year 1","Evening"),
]
for sid, name, course, year, stype in student_display:
    print(f"  {sid:<16} {name:<22} {course:<10} {year:<7} {stype}")
print("  (+ 8 more students — see full list in the DB)")

print()
print("  TOTP SECRETS (scan in your authenticator app):")
print(f"  VC  secret : {vc_totp_secret}")
print(f"  VC  URL    : {vc_url}")
print()
print(f"  DQA secret : {dqa_totp_secret}")
print(f"  DQA URL    : {dqa_url}")
print()
print("  CURRENT TOTP CODES (valid ~30 s from now):")
print(f"  VC  : {pyotp.TOTP(vc_totp_secret).now()}")
print(f"  DQA : {pyotp.TOTP(dqa_totp_secret).now()}")
print()
print("  TODAY's SESSIONS pre-scheduled (PENDING_LECTURER):")
print("  Unit    Course  Coordinator     Venue   Students enrolled")
print("  ------  ------  --------------  ------  -----------------")
print("  CS301   CS Y3   coord.cs        MLH-01  Morning class")
print("  CS201   CS Y2   coord.cs        AUD-01  Evening class")
print("  SE201   SE Y2   coord.se        SEM-01  Morning class")
print("  SE101   SE Y1   coord.se        ICT-01  Distance/Online")
print("  IT301   IT Y3   coord.it        LAB-A1  Weekend class")
print("  IT101   IT Y1   coord.it        SEM-01  Evening class")
print()
print("  DASHBOARDS:")
print("  Admin / VC / DQA / QA  →  http://localhost:3001")
print("  Coordinator PWA         →  http://localhost:3000")
print("  Student Portal          →  http://localhost:3003")
print("  Check-in (any device)   →  http://localhost:8443/checkin")
print("=" * 72)

package handlers

import "net/http"

// CheckinPage serves the student captive-portal check-in page (GET /checkin).
// The student's QR code encodes a URL that includes their signed identity token
// as ?t=<base64url>. Scanning the QR with any phone camera opens this page
// directly — no login, no app, no separate navigation required.
func CheckinPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	// Release the TCP socket promptly — on a small classroom hotspot we don't want
	// idle keep-alive connections lingering while students rotate through.
	w.Header().Set("Connection", "close")
	_, _ = w.Write([]byte(checkinPageHTML))
}

const checkinPageHTML = `<!DOCTYPE html>
<html lang="en"><head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>QAAT — Attendance Check-in</title>
<style>
  :root{--bg:#f1f5f9;--surface:#fff;--text:#0f172a;--muted:#64748b;--border:#e2e8f0;--brand:#2563eb;--brand-weak:#eff6ff;--shadow:0 4px 24px rgba(0,0,0,.08);color-scheme:light}
  :root[data-theme=dark]{--bg:#0b1220;--surface:#111b30;--text:#e2e8f0;--muted:#94a3b8;--border:#25324d;--brand:#3b82f6;--brand-weak:#13233f;--shadow:0 4px 24px rgba(0,0,0,.5);color-scheme:dark}
  *{box-sizing:border-box;margin:0;padding:0}
  body{font-family:system-ui,-apple-system,sans-serif;background:var(--bg);color:var(--text);min-height:100vh;display:flex;align-items:center;justify-content:center;padding:24px 16px;transition:background .2s,color .2s}
  .card{background:var(--surface);border-radius:20px;padding:28px 28px 36px;max-width:380px;width:100%;box-shadow:var(--shadow)}

  /* Brand bar — institution logo + motto, with a light/dark toggle */
  .brandbar{display:flex;align-items:center;justify-content:space-between;margin-bottom:22px}
  .brand{display:flex;align-items:center;gap:10px;min-width:0}
  .brand-logo{height:34px;width:34px;border-radius:8px;background:var(--brand);color:#fff;display:flex;align-items:center;justify-content:center;font-weight:700;flex-shrink:0;overflow:hidden}
  .brand-name{font-weight:700;font-size:14px;color:var(--text);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .brand-motto{font-size:11px;color:var(--muted);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .toggle{height:32px;width:32px;border-radius:8px;background:var(--bg);color:var(--text);border:1px solid var(--border);cursor:pointer;font-size:15px;flex-shrink:0}
  h1{font-size:20px;font-weight:700;color:var(--text);margin-bottom:6px}
  .sub{font-size:13px;color:var(--muted);margin-bottom:20px;line-height:1.5}

  /* Identity badge — shows the QR holder's name before submission */
  .id-badge{display:flex;align-items:center;gap:12px;background:var(--brand-weak);border:1px solid var(--border);border-radius:12px;padding:12px 14px;margin-bottom:22px}
  .id-avatar{width:40px;height:40px;border-radius:50%;background:var(--brand);display:flex;align-items:center;justify-content:center;font-size:16px;font-weight:700;color:#fff;flex-shrink:0}
  .id-info{min-width:0}
  .id-name{font-weight:700;font-size:15px;color:var(--text);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .id-sub{font-size:11px;color:var(--muted);margin-top:2px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .id-warn{font-size:11px;color:#b45309;margin-top:6px;background:#fefce8;border:1px solid #fde68a;border-radius:6px;padding:5px 8px}

  label{display:block;font-size:13px;font-weight:600;color:var(--text);margin-bottom:8px}
  input[type=text]{width:100%;padding:16px;font-size:32px;font-weight:700;letter-spacing:10px;text-align:center;border:2px solid var(--border);border-radius:12px;outline:none;transition:border .15s;font-variant-numeric:tabular-nums;background:var(--surface);color:var(--text)}
  input[type=text]:focus{border-color:var(--brand)}
  button.primary{margin-top:20px;width:100%;padding:15px;font-size:15px;font-weight:700;color:#fff;background:var(--brand);border:0;border-radius:12px;cursor:pointer;transition:filter .15s}
  button.primary:hover:not(:disabled){filter:brightness(.92)}
  button.primary:disabled{background:#94a3b8;cursor:default}
  .error{margin-top:12px;padding:11px 14px;background:#fef2f2;border:1px solid #fecaca;border-radius:10px;color:#b91c1c;font-size:13px;text-align:center;display:none}

  /* Success screen (hidden via inline style like its sibling screens, so show()
     — which clears the inline display — reliably reveals it) */
  .success{text-align:center}
  .check{font-size:56px;margin-bottom:12px}
  .present{font-size:22px;font-weight:700;color:#16a34a;margin-bottom:4px}
  .student-name-big{font-size:17px;font-weight:600;color:var(--text);margin-bottom:14px}
  .unit{font-size:15px;color:var(--text);margin-bottom:3px}
  .meta{font-size:12px;color:var(--muted);margin-bottom:20px}
  .pct-bar{background:var(--border);border-radius:100px;height:8px;margin-bottom:8px;overflow:hidden}
  .pct-fill{height:100%;border-radius:100px;background:var(--brand);transition:width .6s ease}
  .pct-label{font-size:12px;color:var(--muted);margin-bottom:16px;text-align:center}
  /* Disconnect call-to-action — the slot-freeing nudge (manual rotation) */
  .disconnect{background:#fffbeb;border:2px solid #f59e0b;border-radius:14px;padding:16px 14px;margin:8px 0 18px;text-align:center}
  .disc-title{font-size:18px;font-weight:800;color:#b45309;margin-bottom:6px}
  .disc-sub{font-size:13px;color:#92400e;line-height:1.45;margin-bottom:12px}
  .disc-ring{display:inline-flex;align-items:center;justify-content:center;width:54px;height:54px;border-radius:50%;border:3px solid #f59e0b;font-size:20px;font-weight:800;color:#b45309;animation:discpulse 1s infinite}
  @keyframes discpulse{0%,100%{transform:scale(1)}50%{transform:scale(1.09)}}
  button.outline{width:100%;padding:13px;font-size:14px;font-weight:600;color:var(--brand);background:var(--brand-weak);border:2px solid var(--border);border-radius:12px;cursor:pointer}
  button.outline:hover{filter:brightness(.97)}

  .no-token{text-align:center;padding:12px;color:#92400e;background:#fefce8;border:1px solid #fde68a;border-radius:10px;font-size:13px}

  /* Waiting screen — shown when coordinator hasn't started the session yet */
  .waiting{text-align:center;padding:8px}
  .wait-icon{font-size:52px;margin-bottom:12px}
  .wait-title{font-size:19px;font-weight:700;color:var(--text);margin-bottom:8px}
  .wait-sub{font-size:13px;color:var(--muted);margin-bottom:24px;line-height:1.5}
  .retry-ring{display:inline-flex;align-items:center;justify-content:center;width:48px;height:48px;border-radius:50%;border:3px solid var(--border);font-size:18px;font-weight:700;color:var(--brand);margin-bottom:16px}
</style>
</head>
<body>
<div class="card">
  <div class="brandbar">
    <div class="brand">
      <div class="brand-logo" id="brand-logo">Q</div>
      <div style="min-width:0">
        <div class="brand-name" id="brand-name">QAAT</div>
        <div class="brand-motto" id="brand-motto">Attendance</div>
      </div>
    </div>
    <button class="toggle" id="theme-toggle" aria-label="Toggle light or dark mode">&#x263E;</button>
  </div>

  <!-- ── Waiting (coordinator hasn't started session) ──────────────────────── -->
  <div id="screen-waiting" style="display:none">
    <div class="waiting">
      <div class="wait-icon">&#x23F3;</div>
      <div class="wait-title">Session not started yet</div>
      <p class="wait-sub">Your coordinator hasn't opened a session for your course.<br>This page will check automatically.</p>
      <div class="retry-ring" id="retry-ring">15</div>
      <br>
      <button class="outline" id="btn-retry-now" style="max-width:200px;display:inline-block;width:auto;padding:10px 20px">Try now</button>
    </div>
  </div>

  <!-- ── Token missing ─────────────────────────────────────────────────────── -->
  <div id="screen-notoken" style="display:none">
    <h1>Scan your QR code</h1>
    <p class="sub">Open the QR code from your university email and scan it with your camera — this page will open automatically.</p>
    <div class="no-token">No student identity found in this link.</div>
  </div>

  <!-- ── Check-in form ─────────────────────────────────────────────────────── -->
  <div id="screen-form" style="display:none">
    <h1>Enter the code</h1>
    <p class="sub">Type the 6-digit code shown on the coordinator's screen.</p>

    <!-- Identity badge (populated from the QR token client-side) -->
    <div class="id-badge" id="id-badge">
      <div class="id-avatar" id="id-avatar">?</div>
      <div class="id-info">
        <div class="id-name" id="id-name">Loading…</div>
        <div class="id-sub" id="id-sub"></div>
      </div>
    </div>
    <div class="id-warn" id="id-warn" style="display:none">
      &#x26A0;&#xFE0F; This QR belongs to someone else. If this is not your name above, do not proceed.
    </div>

    <label for="code">Room code</label>
    <input type="text" id="code" inputmode="numeric" pattern="[0-9]*" maxlength="6" placeholder="000000" autocomplete="off" autofocus>
    <button class="primary" id="btn-submit">Confirm attendance</button>
    <div class="error" id="err"></div>
  </div>

  <!-- ── Success ───────────────────────────────────────────────────────────── -->
  <div id="screen-success" class="success" style="display:none">
    <div class="check">&#x2705;</div>
    <div class="present">You're marked present</div>
    <div class="student-name-big" id="success-name"></div>

    <!-- The slot-freeing nudge — on a small class hotspot only ~10 phones fit at
         once, so each student must disconnect to let the next one check in. -->
    <div class="disconnect">
      <div class="disc-title">&#x1F4F4; Turn your Wi-Fi OFF now</div>
      <div class="disc-sub">A classmate needs your spot. <strong>Disconnect from the class Wi-Fi</strong> so the next student can check in.</div>
      <div class="disc-ring" id="disc-ring">10</div>
    </div>

    <div class="unit" id="unit-name"></div>
    <div class="meta" id="meta-line"></div>
    <div class="pct-bar"><div class="pct-fill" id="pct-fill" style="width:0%"></div></div>
    <div class="pct-label" id="pct-label"></div>
    <button class="outline" id="btn-history">View attendance history</button>
  </div>
</div>

<script>
const params = new URLSearchParams(location.search);
const token  = params.get('t') || '';

// ── Light / dark theme ────────────────────────────────────────────────────────
(function(){
  var KEY='qaat_theme';
  function initial(){try{var s=localStorage.getItem(KEY);if(s==='light'||s==='dark')return s;}catch(e){}return window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';}
  function apply(t){document.documentElement.dataset.theme=t;try{localStorage.setItem(KEY,t);}catch(e){}var b=document.getElementById('theme-toggle');if(b)b.innerHTML=t==='dark'?'☀':'☾';}
  var cur=initial();apply(cur);
  var btn=document.getElementById('theme-toggle');
  if(btn)btn.addEventListener('click',function(){cur=cur==='dark'?'light':'dark';apply(cur);});
})();

function show(id) {
  ['screen-waiting','screen-notoken','screen-form','screen-success'].forEach(function(s) {
    document.getElementById(s).style.display = s === id ? '' : 'none';
  });
}

// ── Auto-retry when SESSION_NOT_STARTED ───────────────────────────────────────
var retryTimer = null;
var retryInterval = null;

function startRetryCountdown() {
  show('screen-waiting');
  clearRetry();
  var secs = 15;
  document.getElementById('retry-ring').textContent = secs;
  retryInterval = setInterval(function() {
    secs--;
    document.getElementById('retry-ring').textContent = Math.max(secs, 0);
    if (secs <= 0) {
      clearRetry();
      document.getElementById('btn-submit').click();
    }
  }, 1000);
}

function clearRetry() {
  if (retryInterval) { clearInterval(retryInterval); retryInterval = null; }
  if (retryTimer)    { clearTimeout(retryTimer);     retryTimer    = null; }
}

document.getElementById('btn-retry-now').addEventListener('click', function() {
  clearRetry();
  show('screen-form');
  document.getElementById('btn-submit').click();
});

// ── Decode the token to show the student's identity (client-side, display only) ──
// The server re-verifies the RSA signature; we just read the name for display.
function decodeToken(t) {
  try {
    // base64url → base64 → JSON
    var b64 = t.replace(/-/g, '+').replace(/_/g, '/');
    while (b64.length % 4) b64 += '=';
    return JSON.parse(atob(b64));
  } catch(e) { return null; }
}

// ── Institution branding (logo + motto + brand colour) from the token's tenant ──
function loadBranding() {
  var qr = decodeToken(token);
  if (!qr || !qr.tenant_id) return;
  fetch('/api/v1/branding/public?tenant_id=' + encodeURIComponent(qr.tenant_id))
    .then(function(r){ return r.ok ? r.json() : null; })
    .then(function(b){
      if (!b) return;
      var hex = /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/;
      if (hex.test(b.brand_color || '')) document.documentElement.style.setProperty('--brand', b.brand_color);
      if (hex.test(b.background_color || '')) document.documentElement.style.setProperty('--bg', b.background_color);
      document.getElementById('brand-name').textContent = b.name || 'QAAT';
      document.getElementById('brand-motto').textContent = b.motto || 'Attendance';
      var lg = document.getElementById('brand-logo');
      if (b.logo_url) {
        lg.textContent = '';
        var img = document.createElement('img');
        img.src = b.logo_url; img.alt = b.name || '';
        img.style.cssText = 'height:100%;width:100%;object-fit:contain';
        lg.appendChild(img);
      } else { lg.textContent = (b.name || 'Q').slice(0,1); }
    }).catch(function(){});
}
loadBranding();

if (!token) {
  show('screen-notoken');
} else {
  show('screen-form');
  var qr = decodeToken(token);
  if (qr && qr.full_name) {
    var initials = qr.full_name.split(' ').map(function(w){ return w[0]; }).join('').substring(0,2).toUpperCase();
    document.getElementById('id-avatar').textContent = initials;
    document.getElementById('id-name').textContent = qr.full_name;
    document.getElementById('id-sub').textContent = (qr.academic_year || '') + (qr.course_id ? ' · ' + qr.course_id : '');
    // The warning is always shown — a visible reminder not to use someone else's QR.
    document.getElementById('id-warn').style.display = '';
  } else {
    document.getElementById('id-name').textContent = 'Unknown student';
  }
}

// ── Device fingerprint ────────────────────────────────────────────────────────
async function fingerprint() {
  var raw = [
    navigator.userAgent, navigator.language,
    screen.width + 'x' + screen.height + 'x' + screen.colorDepth,
    new Date().getTimezoneOffset(),
    navigator.hardwareConcurrency || 0,
  ].join('|');
  var buf = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(raw));
  return Array.from(new Uint8Array(buf)).map(function(b){ return b.toString(16).padStart(2,'0'); }).join('');
}

function showErr(msg) {
  var el = document.getElementById('err');
  el.textContent = msg;
  el.style.display = msg ? '' : 'none';
}

document.getElementById('btn-submit').addEventListener('click', async function() {
  var code = document.getElementById('code').value.trim();
  if (!/^\d{6}$/.test(code)) { showErr('Please enter the 6-digit code from the screen.'); return; }

  var btn = document.getElementById('btn-submit');
  btn.disabled = true;
  btn.textContent = 'Checking…';
  showErr('');

  try {
    var fp = await fingerprint();
    var res = await fetch('/api/v1/checkin', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ student_token: token, room_code: code, fingerprint: fp }),
    });
    var data = await res.json();

    // SESSION_NOT_STARTED → coordinator hasn't opened a session yet; auto-retry.
    if (data.reason === 'SESSION_NOT_STARTED') {
      btn.disabled = false;
      btn.textContent = 'Confirm attendance';
      startRetryCountdown();
      return;
    }

    if (data.status === 'PRESENT') {
      clearRetry(); // stop any polling so we hold no connection / Wi-Fi slot
      document.getElementById('success-name').textContent = data.student_name || '';
      document.getElementById('unit-name').textContent = data.unit_name || '';
      var meta = [data.lecturer_name, data.session_date].filter(Boolean).join(' · ');
      document.getElementById('meta-line').textContent = meta;
      var pct = Math.min(100, Math.max(0, data.attendance_percent || 0));
      document.getElementById('pct-fill').style.width = pct + '%';
      document.getElementById('pct-label').textContent = 'Attendance this unit: ' + pct + '%';
      show('screen-success');
      startDisconnectCountdown();
      return;
    }

    var reasons = {
      SESSION_NOT_STARTED:            'No active session for your course. The page will retry automatically.',
      PROXIMITY_FAILED:               'Wrong code — check the screen and try again.',
      SESSION_NOT_ACTIVE:             'No active session right now. Ask your coordinator.',
      GATE_NOT_OPEN:                  'The check-in window is closed.',
      NOT_ON_ROSTER:                  'You are not enrolled in the active session.',
      SERIAL_REVOKED:                 'Your QR code has been replaced. Check your email for a new one.',
      DUPLICATE_SCAN:                 'You are already marked present for this session.',
      DEVICE_MISMATCH:                'This device is not your registered phone. Use the device you first used for check-in.',
      DEVICE_ALREADY_USED:            'This device has already been used for a different student this session.',
      DEVICE_BELONGS_TO_ANOTHER_STUDENT: 'This device is registered to a different student. Proxy attendance is not allowed.',
      INVALID_SIGNATURE:              'QR code could not be verified. Ask for a re-issue.',
      QR_EXPIRED:                     'QR code has expired. Ask for a re-issue.',
    };
    showErr(reasons[data.reason] || 'Check-in failed: ' + (data.reason || 'unknown error'));
    btn.disabled = false;
    btn.textContent = 'Confirm attendance';
  } catch(e) {
    showErr('Network error. Check your connection and try again.');
    btn.disabled = false;
    btn.textContent = 'Confirm attendance';
  }
});

document.getElementById('code').addEventListener('keydown', function(e) {
  if (e.key === 'Enter') document.getElementById('btn-submit').click();
});

// ── Disconnect nudge ──────────────────────────────────────────────────────────
// A small hotspot only holds ~10 phones at once, so each student must drop off the
// Wi-Fi to free a slot. The web can't force a disconnect, so we count down and tell
// them clearly. (The native coordinator app surfaces the same message natively.)
function startDisconnectCountdown() {
  var ring = document.getElementById('disc-ring');
  if (!ring) return;
  var s = 10;
  ring.textContent = s;
  var iv = setInterval(function() {
    s--;
    if (s <= 0) {
      clearInterval(iv);
      ring.textContent = '✓';
      ring.style.borderColor = '#16a34a';
      ring.style.color = '#16a34a';
      ring.style.animation = 'none';
      var sub = document.querySelector('.disc-sub');
      if (sub) sub.innerHTML = 'You can <strong>close this page and turn Wi-Fi off</strong> now.';
    } else {
      ring.textContent = s;
    }
  }, 1000);
}

document.getElementById('btn-history').addEventListener('click', function() {
  location.href = '/student/attendance?t=' + encodeURIComponent(token);
});
</script>
</body></html>`

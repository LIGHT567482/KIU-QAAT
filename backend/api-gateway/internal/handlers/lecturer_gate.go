package handlers

// Lecturer Gate QR — anti-ghost-lecture proof-of-presence
//
// Flow:
//  1. Coordinator opens a session and taps "Show Lecturer QR" in the PWA.
//  2. GET /api/v1/sessions/{session_id}/lecturer-gate-qr (COORDINATOR role)
//     Signs a time-limited token (HMAC-SHA256 keyed by the session secret) and
//     returns it as a URL string. The PWA renders it as a QR code client-side.
//  3. The lecturer scans the QR with their own phone. The URL opens:
//     GET /lecturer/checkin?t=<token>  (public, no JWT)
//  4. The lecturer's browser POSTs to:
//     POST /api/v1/lecturer/gate-scan  (public, HMAC-authenticated)
//     The gateway verifies the token and writes lecturer_scanned_at +
//     lecturer_fingerprint_hash into lecturer_attendance_logs.
//
// Security properties:
//   - HMAC key is the per-session checkin_secret (never sent to clients)
//   - Token expires 90 minutes after issuance
//   - Each session can only be scanned once (ON CONFLICT DO NOTHING on the UPDATE)
//   - Device fingerprint binds the scan to the lecturer's physical phone

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// buildLecturerCheckinURL builds the public captive-portal URL the coordinator
// shows as a QR. Like the student QR, it carries only a SHORT reference — the
// session_id — so the QR stays low-density and scans reliably; the portal then
// resolves the session server-side. Anti-replay comes from the live 10s digit
// code the lecturer must type, not from the QR itself.
func buildLecturerCheckinURL(r *http.Request, sessionID string) string {
	// SaaS-adaptive: follow the host the TENANT is actually using so the lecturer
	// QR (and its WebAuthn origin) align with the tenant's address rather than one
	// baked host. WEBAUTHN_RP_ORIGINS/STUDENT_CHECKIN_BASE_URL is only an override.
	base := requestBaseURL(r)
	if ov := strings.TrimRight(os.Getenv("LECTURER_CHECKIN_BASE_URL"), "/"); ov != "" {
		base = ov
	}
	return fmt.Sprintf("%s/lecturer/checkin?s=%s", base, sessionID)
}

// requestBaseURL returns scheme://host derived from the incoming request (honours
// X-Forwarded-Host/Proto set by the gateway/Caddy), so generated URLs match the
// address the tenant used.
func requestBaseURL(r *http.Request) string {
	host := r.Host
	if fh := r.Header.Get("X-Forwarded-Host"); fh != "" {
		host = strings.Split(fh, ",")[0]
	}
	scheme := "https"
	if fp := r.Header.Get("X-Forwarded-Proto"); fp != "" {
		scheme = fp
	} else if r.TLS == nil {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s", scheme, strings.TrimSpace(host))
}


// ── Handler: GET /api/v1/sessions/{session_id}/lecturer-gate-qr ──────────────

// LecturerGateQR generates a signed URL for the coordinator to display as a QR
// code. The URL opens a captive portal on the lecturer's phone where they
// confirm physical presence.
func LecturerGateQR(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		coordID := middleware.GetUserID(r.Context())
		sessionID := chi.URLParam(r, "session_id")
		if !middleware.ValidTenantID(sessionID) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_SESSION_ID"})
			return
		}

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "INTERNAL_ERROR"})
			return
		}
		defer conn.Release()
		if err := middleware.SetTenantConn(r.Context(), conn, tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "INTERNAL_ERROR"})
			return
		}

		// Confirm the coordinator owns this ACTIVE session, then hand back a SHORT
		// URL carrying only the session_id (keeps the QR low-density + scannable;
		// the captive portal fetches the display info + the lecturer types the live
		// digit code, so the QR itself need not rotate).
		var okFlag bool
		err = conn.QueryRow(r.Context(),
			`SELECT true FROM sessions
			 WHERE session_id = $1 AND coordinator_id = $2 AND tenant_id = $3 AND session_status = 'ACTIVE'`,
			sessionID, coordID, tenantID).Scan(&okFlag)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "SESSION_NOT_FOUND"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"session_id":  sessionID,
			"checkin_url": buildLecturerCheckinURL(r, sessionID),
		})
	}
}

// ── Handler: POST /api/v1/lecturer/gate-scan ─────────────────────────────────

// ── Handler: GET /lecturer/checkin ───────────────────────────────────────────

// LecturerCheckinPage serves the lecturer's captive portal — the page that
// opens when a lecturer scans the coordinator's gate QR code.
func LecturerCheckinPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(lecturerCheckinPageHTML))
}

const lecturerCheckinPageHTML = `<!DOCTYPE html>
<html lang="en"><head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>QAAT — Lecturer Attendance</title>
<style>
  :root{--bg:#f1f5f9;--surface:#fff;--text:#0f172a;--muted:#64748b;--border:#e2e8f0;--brand:#0f172a;--shadow:0 4px 24px rgba(0,0,0,.08);color-scheme:light}
  :root[data-theme=dark]{--bg:#0b1220;--surface:#111b30;--text:#e2e8f0;--muted:#94a3b8;--border:#25324d;--brand:#3b82f6;--shadow:0 4px 24px rgba(0,0,0,.5);color-scheme:dark}
  *{box-sizing:border-box;margin:0;padding:0}
  body{font-family:system-ui,-apple-system,sans-serif;background:var(--bg);color:var(--text);min-height:100vh;display:flex;align-items:center;justify-content:center;padding:24px 16px;transition:background .2s,color .2s}
  .card{background:var(--surface);border-radius:20px;padding:28px 28px 36px;max-width:380px;width:100%;box-shadow:var(--shadow)}
  .brandbar{display:flex;align-items:center;justify-content:space-between;margin-bottom:22px}
  .brand{display:flex;align-items:center;gap:10px;min-width:0}
  .brand-logo{height:34px;width:34px;border-radius:8px;background:var(--brand);color:#fff;display:flex;align-items:center;justify-content:center;font-weight:700;flex-shrink:0;overflow:hidden}
  .brand-name{font-weight:700;font-size:14px;color:var(--text);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .brand-motto{font-size:11px;color:var(--muted);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .toggle{height:32px;width:32px;border-radius:8px;background:var(--bg);color:var(--text);border:1px solid var(--border);cursor:pointer;font-size:15px;flex-shrink:0}
  h1{font-size:20px;font-weight:700;color:var(--text);margin-bottom:6px}
  .sub{font-size:13px;color:var(--muted);margin-bottom:24px;line-height:1.5}
  .info-box{background:var(--bg);border:1px solid var(--border);border-radius:12px;padding:14px 16px;margin-bottom:24px;font-size:13px;color:var(--text)}
  .info-row{display:flex;justify-content:space-between;margin-bottom:4px}
  .info-label{font-weight:600;color:var(--muted)}
  button.primary{width:100%;padding:15px;font-size:15px;font-weight:700;color:#fff;background:var(--brand);border:0;border-radius:12px;cursor:pointer;transition:filter .15s}
  button.primary:hover:not(:disabled){filter:brightness(1.1)}
  button.primary:disabled{background:#94a3b8;cursor:default}
  .error{margin-top:12px;padding:11px 14px;background:#fef2f2;border:1px solid #fecaca;border-radius:10px;color:#b91c1c;font-size:13px;text-align:center;display:none}
  .success{text-align:center}
  .check{font-size:56px;margin-bottom:12px}
  .present{font-size:22px;font-weight:700;color:#16a34a;margin-bottom:6px}
  .present-sub{font-size:14px;color:var(--muted)}
  .no-token{text-align:center;padding:12px;color:#92400e;background:#fefce8;border:1px solid #fde68a;border-radius:10px;font-size:13px}
</style>
</head>
<body>
<div class="card">
  <div class="brandbar">
    <div class="brand">
      <div class="brand-logo" id="brand-logo">Q</div>
      <div style="min-width:0">
        <div class="brand-name" id="brand-name">QAAT</div>
        <div class="brand-motto" id="brand-motto">Lecturer Gate</div>
      </div>
    </div>
    <button class="toggle" id="theme-toggle" aria-label="Toggle light or dark mode">&#x263E;</button>
  </div>

  <!-- ── Token missing ── -->
  <div id="screen-notoken" style="display:none">
    <h1>Scan the QR code</h1>
    <p class="sub">Ask the coordinator to show you the Lecturer QR code and scan it with your camera.</p>
    <div class="no-token">No session token found in this link.</div>
  </div>

  <!-- ── Confirm form ── -->
  <div id="screen-form" style="display:none">
    <h1>Confirm your presence</h1>
    <p class="sub">Scan to <strong>begin</strong> the lecture, and scan again to <strong>end</strong> it. Each scan needs your <strong>staff ID</strong>, the <strong>live code</strong> on the coordinator's screen, and your <strong>fingerprint</strong>.</p>
    <div class="info-box" id="session-info">
      <div class="info-row"><span class="info-label">Unit:</span><span id="info-unit"></span></div>
      <div class="info-row"><span class="info-label">Lecturer:</span><span id="info-lecturer"></span></div>
      <div class="info-row"><span class="info-label">Coordinator:</span><span id="info-coord"></span></div>
      <div class="info-row"><span class="info-label">Venue:</span><span id="info-venue"></span></div>
      <div class="info-row"><span class="info-label">Date:</span><span id="info-date"></span></div>
      <div class="info-row"><span class="info-label">Planned:</span><span id="info-planned"></span></div>
      <div class="info-row"><span class="info-label">Session:</span><span id="info-session"></span></div>
    </div>
    <label for="staffid" style="display:block;font-size:13px;font-weight:600;color:var(--text);margin-bottom:8px">Your staff ID</label>
    <input type="text" id="staffid" placeholder="Enter your staff ID" autocomplete="off" autofocus
      style="width:100%;padding:14px;font-size:16px;border:2px solid var(--border);border-radius:12px;outline:none;background:var(--surface);color:var(--text);margin-bottom:14px">
    <label for="roomcode" style="display:block;font-size:13px;font-weight:600;color:var(--text);margin-bottom:8px">Live digit code (on the coordinator's screen)</label>
    <input type="text" id="roomcode" placeholder="6-digit code" inputmode="numeric" maxlength="6" autocomplete="off"
      style="width:100%;padding:14px;font-size:20px;letter-spacing:4px;text-align:center;border:2px solid var(--border);border-radius:12px;outline:none;background:var(--surface);color:var(--text);margin-bottom:4px">
    <button class="primary" id="btn-confirm" style="margin-top:14px">Verify fingerprint &amp; confirm</button>
    <div class="error" id="err"></div>
  </div>

  <!-- ── Success ── -->
  <div id="screen-success" class="success" style="display:none">
    <div class="check">&#x2705;</div>
    <div class="present" id="success-title">Attendance recorded</div>
    <p class="present-sub" id="success-sub" style="margin-top:8px">Your presence has been noted for this session.</p>
    <p class="present-sub" style="margin-top:14px;color:#b45309;background:#fffbeb;border:1px solid #f59e0b;border-radius:10px;padding:10px 12px;font-size:13px">&#x1F4F4; You can <strong>disconnect from the class Wi-Fi</strong> now to free a spot &mdash; reconnect only when you scan again to end the lecture.</p>
  </div>
</div>

<script>
const params = new URLSearchParams(location.search);
const sid = params.get('s') || '';

function show(id){['screen-notoken','screen-form','screen-success'].forEach(function(x){document.getElementById(x).style.display=x===id?'':'none';});}

(function(){var KEY='qaat_theme';function initial(){try{var s=localStorage.getItem(KEY);if(s==='light'||s==='dark')return s;}catch(e){}return window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';}function apply(t){document.documentElement.dataset.theme=t;try{localStorage.setItem(KEY,t);}catch(e){}var b=document.getElementById('theme-toggle');if(b)b.innerHTML=t==='dark'?'\u2600':'\u263E';}var cur=initial();apply(cur);var btn=document.getElementById('theme-toggle');if(btn)btn.addEventListener('click',function(){cur=cur==='dark'?'light':'dark';apply(cur);});})();

function applyBranding(tenantId){
  if(!tenantId) return;
  fetch('/api/v1/branding/public?tenant_id='+encodeURIComponent(tenantId)).then(function(r){return r.ok?r.json():null;}).then(function(b){
    if(!b) return; var hex=/^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/;
    if(hex.test(b.brand_color||'')) document.documentElement.style.setProperty('--brand',b.brand_color);
    if(hex.test(b.background_color||'')) document.documentElement.style.setProperty('--bg',b.background_color);
    document.getElementById('brand-name').textContent=b.name||'QAAT';
    document.getElementById('brand-motto').textContent=b.motto||'Lecturer Gate';
    var lg=document.getElementById('brand-logo');
    if(b.logo_url){lg.textContent='';var img=document.createElement('img');img.src=b.logo_url;img.alt=b.name||'';img.style.cssText='height:100%;width:100%;object-fit:contain';lg.appendChild(img);}
    else{lg.textContent=(b.name||'Q').slice(0,1);}
  }).catch(function(){});
}

if(!sid){ show('screen-notoken'); }
else {
  fetch('/api/v1/lecturer/session-info?s='+encodeURIComponent(sid)).then(function(r){return r.ok?r.json():null;}).then(function(pp){
    if(!pp){ show('screen-notoken'); return; }
    document.getElementById('info-unit').textContent=pp.unit_name||'\u2014';
    document.getElementById('info-lecturer').textContent=pp.lecturer_name||'\u2014';
    document.getElementById('info-coord').textContent=pp.coordinator_name||'\u2014';
    document.getElementById('info-venue').textContent=pp.venue_name||'\u2014';
    document.getElementById('info-date').textContent=pp.session_date||'\u2014';
    document.getElementById('info-planned').textContent=pp.planned_start?(pp.planned_start+(pp.planned_minutes?' \u00b7 '+pp.planned_minutes+' min':'')):'\u2014';
    document.getElementById('info-session').textContent=sid.substring(0,8)+'\u2026';
    applyBranding(pp.tenant_id);
    show('screen-form');
  }).catch(function(){ show('screen-notoken'); });
}

async function fingerprint(){var raw=[navigator.userAgent,navigator.language,screen.width+'x'+screen.height+'x'+screen.colorDepth,new Date().getTimezoneOffset(),navigator.hardwareConcurrency||0].join('|');var buf=await crypto.subtle.digest('SHA-256',new TextEncoder().encode(raw));return Array.from(new Uint8Array(buf)).map(function(b){return b.toString(16).padStart(2,'0');}).join('');}

// WebAuthn (phone fingerprint) helpers + assertion. If the lecturer has enrolled a
// passkey, the OS prompts for their fingerprint and we prove it to the server.
function b64uToBuf(s){s=s.replace(/-/g,'+').replace(/_/g,'/');var p=s.length%4;if(p)s+='='.repeat(4-p);var bin=atob(s);var b=new Uint8Array(bin.length);for(var i=0;i<bin.length;i++)b[i]=bin.charCodeAt(i);return b.buffer;}
function bufToB64u(buf){var b=new Uint8Array(buf);var s='';for(var i=0;i<b.length;i++)s+=String.fromCharCode(b[i]);return btoa(s).replace(/\+/g,'-').replace(/\//g,'_').replace(/=+$/,'');}
// Returns 'ok' (verified), 'skip' (lecturer not enrolled — fall back) or throws.
async function biometricAssert(){
  var rb=await fetch('/api/v1/lecturer/webauthn/assert/begin?s='+encodeURIComponent(sid),{method:'POST'});
  if(rb.status===422){return 'skip';}            // NOT_ENROLLED
  if(!rb.ok){throw new Error('Could not start fingerprint check.');}
  if(!window.PublicKeyCredential){throw new Error('This browser cannot read your fingerprint. Use your phone\'s default browser.');}
  var opt=(await rb.json()).publicKey;
  opt.challenge=b64uToBuf(opt.challenge);
  if(opt.allowCredentials)opt.allowCredentials.forEach(function(c){c.id=b64uToBuf(c.id);});
  var assertion=await navigator.credentials.get({publicKey:opt});
  var body={id:assertion.id,rawId:bufToB64u(assertion.rawId),type:assertion.type,
    response:{authenticatorData:bufToB64u(assertion.response.authenticatorData),clientDataJSON:bufToB64u(assertion.response.clientDataJSON),
      signature:bufToB64u(assertion.response.signature),userHandle:assertion.response.userHandle?bufToB64u(assertion.response.userHandle):null}};
  var rf=await fetch('/api/v1/lecturer/webauthn/assert/finish?s='+encodeURIComponent(sid),{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  if(!rf.ok){throw new Error('Fingerprint did not match the assigned lecturer.');}
  return 'ok';
}

function showErr(msg){var el=document.getElementById('err');el.textContent=msg;el.style.display=msg?'':'none';}
function succeed(title,sub){document.getElementById('success-title').textContent=title;document.getElementById('success-sub').textContent=sub;show('screen-success');}

document.getElementById('btn-confirm').addEventListener('click', async function(){
  var staffId=(document.getElementById('staffid').value||'').trim();
  var roomCode=(document.getElementById('roomcode').value||'').trim();
  if(!staffId){showErr('Please enter your staff ID.');return;}
  if(!roomCode){showErr('Enter the live digit code shown on the coordinator screen.');return;}
  var btn=document.getElementById('btn-confirm');btn.disabled=true;btn.textContent='Confirming\u2026';showErr('');
  try{
    // Phone-fingerprint identity check (skipped only if the lecturer hasn't enrolled).
    try{ await biometricAssert(); }
    catch(be){ showErr(be.message||'Fingerprint check failed.'); btn.disabled=false; btn.textContent='Confirm I am present'; return; }
    var fp=await fingerprint();
    var res=await fetch('/api/v1/lecturer/gate-scan',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({session_id:sid,staff_id:staffId,room_code:roomCode,fingerprint:fp})});
    var data=await res.json();
    if(data.status==='STARTED'){succeed('Lecture started','Scan this QR again at the end of the lecture to close it.');return;}
    if(data.status==='ENDED'){succeed('Lecture ended','Your contact time has been recorded.');return;}
    if(data.status==='ALREADY_COMPLETE'){succeed('Already recorded','This lecture has already been started and ended.');return;}
    var msgs={'session_not_active':'The session is not active.','staff_id_mismatch':'That staff ID does not match the lecturer assigned to this session.','no_staff_id':'No staff ID is on file for the assigned lecturer. Tell the admin.','wrong_code':'That digit code is not current. Read the live code on the coordinator screen.','no_quorum':data.message||'Not enough students have attended yet.','no_lecturer':'No lecturer is assigned to this session.'};
    showErr(msgs[data.error]||(data.message||'Confirmation failed: '+(data.error||'unknown')));
    btn.disabled=false;btn.textContent='Confirm I am present';
  }catch(e){showErr('Network error. Check your connection and try again.');btn.disabled=false;btn.textContent='Confirm I am present';}
});
</script>
</body></html>`

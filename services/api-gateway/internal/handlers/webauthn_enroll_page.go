package handlers

import "net/http"

// LecturerEnrollPage serves the page a lecturer opens (on their own phone) from
// the admin-issued enrolment link to register their fingerprint passkey.
func LecturerEnrollPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(lecturerEnrollPageHTML))
}

const lecturerEnrollPageHTML = `<!DOCTYPE html>
<html lang="en"><head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>QAAT — Enroll Fingerprint</title>
<style>
  :root{--bg:#f1f5f9;--surface:#fff;--text:#0f172a;--muted:#64748b;--border:#e2e8f0;--brand:#0f172a;color-scheme:light}
  :root[data-theme=dark]{--bg:#0b1220;--surface:#111b30;--text:#e2e8f0;--muted:#94a3b8;--border:#25324d;--brand:#3b82f6;color-scheme:dark}
  *{box-sizing:border-box;margin:0;padding:0}
  body{font-family:system-ui,-apple-system,sans-serif;background:var(--bg);color:var(--text);min-height:100vh;display:flex;align-items:center;justify-content:center;padding:24px 16px}
  .card{background:var(--surface);border-radius:20px;padding:30px 26px;max-width:380px;width:100%;box-shadow:0 4px 24px rgba(0,0,0,.08);text-align:center}
  .fp{font-size:60px;margin-bottom:8px}
  h1{font-size:21px;margin-bottom:8px}
  p.sub{font-size:14px;color:var(--muted);margin-bottom:22px;line-height:1.5}
  button{width:100%;padding:15px;font-size:16px;font-weight:700;color:#fff;background:var(--brand);border:0;border-radius:12px;cursor:pointer}
  button:disabled{background:#94a3b8}
  .msg{margin-top:14px;font-size:14px;padding:11px;border-radius:10px;display:none}
  .err{background:#fef2f2;color:#b91c1c}
  .ok{background:#f0fdf4;color:#166534}
</style></head>
<body>
<div class="card">
  <div class="fp">🔒</div>
  <h1 id="title">Enroll your fingerprint</h1>
  <p class="sub" id="sub">Register this phone's fingerprint (or Face unlock) so you can confirm your identity when taking lecture attendance. Your fingerprint never leaves your phone.</p>
  <button id="go">Enroll fingerprint</button>
  <div class="msg err" id="err"></div>
  <div class="msg ok" id="ok"></div>
</div>
<script>
var tk = new URLSearchParams(location.search).get('tk') || '';
function b64uToBuf(s){s=s.replace(/-/g,'+').replace(/_/g,'/');var pad=s.length%4;if(pad)s+='='.repeat(4-pad);var bin=atob(s);var b=new Uint8Array(bin.length);for(var i=0;i<bin.length;i++)b[i]=bin.charCodeAt(i);return b.buffer;}
function bufToB64u(buf){var b=new Uint8Array(buf);var s='';for(var i=0;i<b.length;i++)s+=String.fromCharCode(b[i]);return btoa(s).replace(/\+/g,'-').replace(/\//g,'_').replace(/=+$/,'');}
function showErr(m){var e=document.getElementById('err');e.textContent=m;e.style.display='block';}
function showOk(m){var e=document.getElementById('ok');e.textContent=m;e.style.display='block';}

if(!tk){ showErr('This enrolment link is missing its token. Ask the admin to re-issue it.'); document.getElementById('go').disabled=true; }
if(!window.PublicKeyCredential){ showErr('This browser does not support passkeys. Use your phone\'s default browser over HTTPS.'); }

document.getElementById('go').addEventListener('click', async function(){
  var btn=this; btn.disabled=true; btn.textContent='Follow the prompt…';
  document.getElementById('err').style.display='none';
  try{
    var r=await fetch('/api/v1/lecturer/webauthn/enroll/begin',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({token:tk})});
    if(!r.ok){var e=await r.json().catch(function(){return{};});throw new Error(e.message||('begin failed '+r.status));}
    var opt=(await r.json()).publicKey;
    opt.challenge=b64uToBuf(opt.challenge);
    opt.user.id=b64uToBuf(opt.user.id);
    if(opt.excludeCredentials)opt.excludeCredentials.forEach(function(c){c.id=b64uToBuf(c.id);});
    var cred=await navigator.credentials.create({publicKey:opt});
    var body={id:cred.id,rawId:bufToB64u(cred.rawId),type:cred.type,
      response:{attestationObject:bufToB64u(cred.response.attestationObject),clientDataJSON:bufToB64u(cred.response.clientDataJSON)}};
    var f=await fetch('/api/v1/lecturer/webauthn/enroll/finish?tk='+encodeURIComponent(tk),{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
    if(!f.ok){var e2=await f.json().catch(function(){return{};});throw new Error(e2.message||('finish failed '+f.status));}
    document.getElementById('title').textContent='Fingerprint enrolled ✓';
    document.getElementById('sub').textContent='You can now confirm your identity by fingerprint when you scan a lecture QR code.';
    btn.style.display='none'; showOk('All set — you may close this page.');
  }catch(e){ showErr(e.message||'Enrolment failed. Please try again.'); btn.disabled=false; btn.textContent='Enroll fingerprint'; }
});
</script>
</body></html>`

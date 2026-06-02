package handlers

import "net/http"

// CheckinPage serves the student check-in web app (GET /checkin). It is a single
// self-contained HTML page so it works on any phone with no install. Under HTTPS
// it is a secure context, so crypto.subtle / BarcodeDetector / geolocation are
// all available — the reason the online flow works on iOS where the LAN path did
// not. The page is dumb: it decodes the student's personal QR, computes a device
// fingerprint, and POSTs to /api/v1/checkin, which does all validation.
func CheckinPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(checkinPageHTML))
}

const checkinPageHTML = `<!DOCTYPE html>
<html lang="en"><head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>QAAT Attendance Check-in</title>
<style>
  body{font-family:system-ui,sans-serif;max-width:420px;margin:0 auto;padding:24px;color:#0f172a}
  h1{font-size:20px} label{display:block;margin:16px 0 6px;font-weight:600}
  input{width:100%;box-sizing:border-box;padding:12px;font-size:16px;border:1px solid #cbd5e1;border-radius:8px}
  #code{letter-spacing:4px;text-align:center;font-size:24px}
  button{margin-top:20px;width:100%;padding:14px;font-size:16px;font-weight:700;color:#fff;background:#2563eb;border:0;border-radius:8px}
  button:disabled{background:#94a3b8}
  #result{margin-top:20px;font-size:18px;font-weight:700;text-align:center;min-height:24px}
  small{color:#64748b}
</style></head><body>
<h1>Attendance Check-in</h1>
<form id="f">
  <label for="qr">1. Your QR code (scan or upload)</label>
  <input type="file" id="qr" accept="image/*" capture="environment" required>
  <label for="code">2. Room code on the screen</label>
  <input type="text" id="code" inputmode="numeric" pattern="[0-9]*" maxlength="6" placeholder="000000" required>
  <button type="submit" id="go">Check in</button>
</form>
<div id="result"></div>
<small id="hint"></small>
<script>
const params = new URLSearchParams(location.search);
const sessionId = params.get('session') || '';
const resultEl = document.getElementById('result');
const hintEl = document.getElementById('hint');
if (!sessionId) hintEl.textContent = 'No session in the link — ask the coordinator for the check-in URL/QR.';

async function sha256Hex(s){
  const b = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(s));
  return Array.from(new Uint8Array(b)).map(x=>x.toString(16).padStart(2,'0')).join('');
}
// Same fingerprint formula as apps/coordinator-pwa/src/crypto/fingerprint.ts.
async function fingerprint(){
  const c=document.createElement('canvas'),ctx=c.getContext('2d');
  ctx.textBaseline='top';ctx.font='14px Arial';ctx.fillText('QAAT-fingerprint-probe',2,2);
  const canvasHash=await sha256Hex(c.toDataURL());
  return sha256Hex([navigator.userAgent,screen.width+'x'+screen.height,String(screen.colorDepth),
    String(window.devicePixelRatio),String(navigator.hardwareConcurrency),canvasHash].join('|'));
}
async function decodeQR(file){
  if(!('BarcodeDetector' in window)) return null;
  const bitmap=await createImageBitmap(file);
  const det=new BarcodeDetector({formats:['qr_code']});
  const codes=await det.detect(bitmap); bitmap.close();
  return codes.length?codes[0].rawValue:null;
}
function show(ok,msg){resultEl.textContent=(ok?'✓ ':'✗ ')+msg;resultEl.style.color=ok?'#166534':'#b91c1c';}

document.getElementById('f').addEventListener('submit',async e=>{
  e.preventDefault();
  const go=document.getElementById('go'); go.disabled=true;
  resultEl.textContent='Checking…'; resultEl.style.color='#64748b';
  try{
    const file=document.getElementById('qr').files[0];
    const raw=await decodeQR(file);
    if(!raw){show(false,'Could not read that QR image. Try a clearer photo.');go.disabled=false;return;}
    const body={session_id:sessionId,qr:raw,room_code:document.getElementById('code').value.trim(),fingerprint:await fingerprint()};
    // Retry transient network failures a few times (handles a brief signal drop).
    let res,attempt=0;
    while(true){
      try{res=await fetch('/api/v1/checkin',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});break;}
      catch(err){if(++attempt>=4){show(false,'Network error — move closer to signal and retry.');go.disabled=false;return;}await new Promise(r=>setTimeout(r,attempt*800));}
    }
    const data=await res.json();
    if(data.status==='PRESENT'){show(true,'Checked in');}
    else{show(false,(data.reason||'Rejected').replace(/_/g,' ').toLowerCase());go.disabled=false;}
  }catch(err){show(false,'Something went wrong. Try again.');go.disabled=false;}
});
</script>
</body></html>`

package app

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/matthewlu070111/anpanel/internal/config"
)

const entryCookie = "anpanel_gate"

func (s *server) entryConfigured() bool {
	return strings.TrimSpace(s.cfg.EntryPath) != ""
}

func (s *server) entryPrefix() string {
	p := config.NormalizeEntryPath(s.cfg.EntryPath)
	if p == "" {
		return ""
	}
	return "/" + p
}

func (s *server) gateToken() string {
	key, _ := os.ReadFile(s.cfg.SessionKeyFile)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("entry:" + s.cfg.EntryPath))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *server) validGateCookie(r *http.Request) bool {
	c, err := r.Cookie(entryCookie)
	if err != nil || c.Value == "" {
		return false
	}
	return hmac.Equal([]byte(c.Value), []byte(s.gateToken()))
}

func (s *server) setGateCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     entryCookie,
		Value:    s.gateToken(),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil,
		Expires:  time.Now().Add(30 * 24 * time.Hour),
	})
}

// entryGate enforces secret entry path when configured.
// Visiting /{entry} unlocks a gate cookie; other public hits get the decoy page.
func (s *server) entryGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix := s.entryPrefix()
		if prefix == "" {
			// Not configured yet: panel is open until admin sets entry path after login.
			next.ServeHTTP(w, r)
			return
		}

		path := r.URL.Path
		// Unlock via secret entry URL.
		if path == prefix || path == prefix+"/" {
			s.setGateCookie(w, r)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		if s.validGateCookie(r) {
			next.ServeHTTP(w, r)
			return
		}

		// No cookie: serve decoy for everything (including API) to avoid leaking the panel.
		s.serveDecoy(w, r)
	})
}

func (s *server) serveDecoy(w http.ResponseWriter, r *http.Request) {
	mode := s.cfg.DecoyMode
	if mode != "dino" {
		mode = "404"
	}
	if mode == "dino" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(dinoHTML))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(notFoundHTML))
}

func (s *server) saveEntrySettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Path  string `json:"path"`
		Decoy string `json:"decoy_mode"`
	}
	if !decode(w, r, &in) {
		return
	}
	path := config.NormalizeEntryPath(in.Path)
	if path == "" {
		apiError(w, 400, "invalid entry path: use 4–64 letters/digits/_- (not api/assets)")
		return
	}
	decoy := strings.ToLower(strings.TrimSpace(in.Decoy))
	if decoy != "dino" {
		decoy = "404"
	}
	// Disallow redirect decoys explicitly.
	if decoy == "301" || decoy == "302" || decoy == "redirect" {
		apiError(w, 400, "redirect decoy modes are not allowed")
		return
	}
	cfg, err := config.Load()
	if err != nil {
		apiError(w, 500, err.Error())
		return
	}
	cfg.EntryPath = path
	cfg.DecoyMode = decoy
	if err := config.Save(cfg); err != nil {
		apiError(w, 500, err.Error())
		return
	}
	s.cfg = cfg
	ss := current(r)
	_ = s.db.Audit(ss.User.Username, "settings.entry", path, "decoy="+decoy, remoteIP(r))
	// Issue gate cookie immediately so the current browser keeps working.
	s.setGateCookie(w, r)
	apiJSON(w, map[string]any{
		"ok":           true,
		"entry_path":   path,
		"decoy_mode":   decoy,
		"entry_url":    "/" + path,
		"must_set_entry": false,
	})
}

func randomEntryPath() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

const notFoundHTML = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>404 Not Found</title>
<style>
body{margin:0;min-height:100vh;display:grid;place-items:center;font-family:system-ui,sans-serif;background:#fafafa;color:#222}
main{text-align:center;padding:24px}
h1{font-size:72px;margin:0;font-weight:700;letter-spacing:-2px}
p{color:#666;margin:12px 0 0}
</style></head><body><main><h1>404</h1><p>Not Found</p></main></body></html>`

// Minimal offline-runner style game (inspired by browser offline dino; original tiny implementation).
const dinoHTML = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>No connection</title>
<style>
*{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;place-items:center;background:#f7f7f7;font-family:system-ui,sans-serif;color:#535353}
.wrap{width:min(680px,94vw);text-align:center}
h1{font-size:18px;font-weight:500;margin:0 0 8px}
p{margin:0 0 16px;font-size:13px;color:#70757a}
canvas{width:100%;height:auto;background:#fff;border:1px solid #e5e5e5;border-radius:4px;image-rendering:pixelated;cursor:pointer}
.hint{margin-top:10px;font-size:12px;color:#9aa0a6}
</style></head><body>
<div class="wrap">
  <h1>无法访问此网站</h1>
  <p>请检查网络连接，或按空格 / 点击开始小游戏</p>
  <canvas id="c" width="680" height="180"></canvas>
  <div class="hint">SPACE / CLICK · 跳跃</div>
</div>
<script>
const canvas=document.getElementById('c'),ctx=canvas.getContext('2d');
let t=0,vy=0,y=0,alive=true,score=0,obstacles=[],started=false;
const ground=150, runner={x:48,w:22,h:28};
function reset(){t=0;vy=0;y=0;alive=true;score=0;obstacles=[];started=true;spawn()}
function spawn(){obstacles.push({x:720+Math.random()*80,w:14+Math.random()*10,h:20+Math.random()*18})}
function jump(){if(!started){reset();return}if(alive&&y===0)vy=-9.2}
window.addEventListener('keydown',e=>{if(e.code==='Space'||e.code==='ArrowUp'){e.preventDefault();jump()}});
canvas.addEventListener('pointerdown',jump);
function loop(){
  t++;
  if(started&&alive){
    vy+=0.45;y+=vy;if(y>0){y=0;vy=0}
    if(t%90===0)spawn();
    for(const o of obstacles){o.x-=5}
    obstacles=obstacles.filter(o=>o.x>-40);
    score=Math.floor(t/6);
    for(const o of obstacles){
      const rx=runner.x,ry=ground-runner.h-y,rw=runner.w,rh=runner.h;
      if(rx<o.x+o.w&&rx+rw>o.x&&ry<ground-o.h&&ry+rh>ground-o.h){alive=false}
    }
  }
  ctx.clearRect(0,0,canvas.width,canvas.height);
  ctx.strokeStyle='#535353';ctx.lineWidth=2;ctx.beginPath();ctx.moveTo(0,ground);ctx.lineTo(canvas.width,ground);ctx.stroke();
  ctx.fillStyle='#535353';
  ctx.fillRect(runner.x,ground-runner.h-y,runner.w,runner.h);
  ctx.fillRect(runner.x+4,ground-runner.h-y-6,10,6);
  for(const o of obstacles){ctx.fillRect(o.x,ground-o.h,o.w,o.h)}
  ctx.font='14px monospace';ctx.fillText(String(score).padStart(5,'0'), canvas.width-70, 24);
  if(!started){ctx.fillText('PRESS SPACE', canvas.width/2-48, 80)}
  if(started&&!alive){ctx.fillText('GAME OVER', canvas.width/2-42, 80);ctx.fillText('SPACE TO RETRY', canvas.width/2-58, 100)}
  requestAnimationFrame(loop);
}
loop();
</script></body></html>`

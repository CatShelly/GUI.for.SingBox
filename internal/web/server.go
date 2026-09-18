package web

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"guiforcores/bridge"
	"guiforcores/internal/events"
	"io/fs"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"
)

const sessionLifetime = 30 * 24 * time.Hour

type Server struct {
	app           *bridge.App
	core          *Core
	static        fs.FS
	password      [32]byte
	secure        bool
	mu            sync.Mutex
	sessions      map[string]time.Time
	loginAttempts []time.Time
	closed        chan struct{}
	closeOnce     sync.Once
}

func New(app *bridge.App, static fs.FS, corePath, password string, secure bool) (*Server, error) {
	if password == "" {
		return nil, fmt.Errorf("password is required")
	}
	if _, err := fs.Stat(static, "index.html"); err != nil {
		return nil, fmt.Errorf("frontend index.html: %w", err)
	}
	s := &Server{app: app, static: static, password: sha256.Sum256([]byte(password)), secure: secure, sessions: map[string]time.Time{}, closed: make(chan struct{})}
	s.core = NewCore(app, corePath)
	if err := s.core.Restore(); err != nil {
		s.core.mu.Lock()
		s.core.lastError = err.Error()
		s.core.mu.Unlock()
	}
	return s, nil
}
func (s *Server) Close() { s.closeOnce.Do(func() { close(s.closed); s.core.Shutdown() }) }
func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
func fail(w http.ResponseWriter, status int, err any) {
	respond(w, status, map[string]any{"message": fmt.Sprint(err)})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	return json.NewDecoder(r.Body).Decode(v)
}
func (s *Server) authenticated(r *http.Request) bool {
	cookie, err := r.Cookie("webui_session")
	if err != nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	until, ok := s.sessions[cookie.Value]
	if ok && time.Now().Before(until) {
		return true
	}
	delete(s.sessions, cookie.Value)
	return false
}
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("GET /api/session", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]bool{"authenticated": s.authenticated(r)})
	})
	api := http.NewServeMux()
	api.HandleFunc("POST /api/logout", func(w http.ResponseWriter, r *http.Request) {
		c, _ := r.Cookie("webui_session")
		s.mu.Lock()
		delete(s.sessions, c.Value)
		s.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "webui_session", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: s.secure})
		respond(w, 200, true)
	})
	api.HandleFunc("POST /api/rpc/{method}", s.rpc)
	api.HandleFunc("GET /api/events", s.eventStream)
	api.HandleFunc("POST /api/events", func(w http.ResponseWriter, r *http.Request) {
		var e events.Event
		if err := decode(w, r, &e); err != nil {
			fail(w, 400, err)
			return
		}
		events.EventsEmit(s.app.Ctx, e.Name, e.Data...)
		respond(w, 200, true)
	})
	api.HandleFunc("GET /api/core/status", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, s.core.Status()) })
	api.HandleFunc("POST /api/core/apply", func(w http.ResponseWriter, r *http.Request) {
		var in CoreInput
		if err := decode(w, r, &in); err != nil {
			fail(w, 400, err)
			return
		}
		if err := s.core.Apply(in); err != nil {
			fail(w, 409, err)
			return
		}
		respond(w, 200, s.core.Status())
	})
	api.HandleFunc("POST /api/core/stop", func(w http.ResponseWriter, r *http.Request) {
		if err := s.core.Stop(); err != nil {
			fail(w, 409, err)
			return
		}
		respond(w, 200, s.core.Status())
	})
	api.HandleFunc("POST /api/core/restart", func(w http.ResponseWriter, r *http.Request) {
		if err := s.core.Restart(); err != nil {
			fail(w, 409, err)
			return
		}
		respond(w, 200, s.core.Status())
	})
	api.HandleFunc("/api/core/proxy/", s.core.Proxy(false))
	api.HandleFunc("/api/dashboard/", s.core.Proxy(true))
	mux.Handle("/api/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !s.authenticated(r) {
			fail(w, 401, "Login required")
			return
		}
		if !sameOrigin(r) || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			fail(w, 403, "Origin rejected")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && !strings.HasPrefix(r.URL.Path, "/api/dashboard/") && r.Header.Get("X-WebUI-Request") != "1" {
			fail(w, 403, "Missing request header")
			return
		}
		api.ServeHTTP(w, r)
	}))
	files := http.FileServer(http.FS(s.static))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		mux.ServeHTTP(w, r)
	})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) || r.Header.Get("X-WebUI-Request") != "1" {
		fail(w, 403, "Origin rejected")
		return
	}
	s.mu.Lock()
	now := time.Now()
	recent := s.loginAttempts[:0]
	for _, t := range s.loginAttempts {
		if now.Sub(t) < time.Minute {
			recent = append(recent, t)
		}
	}
	s.loginAttempts = recent
	if len(recent) >= 20 {
		s.mu.Unlock()
		fail(w, 429, "Too many attempts; retry in one minute")
		return
	}
	s.loginAttempts = append(s.loginAttempts, now)
	s.mu.Unlock()
	var in struct {
		Password string `json:"password"`
	}
	if err := decode(w, r, &in); err != nil {
		fail(w, 400, err)
		return
	}
	sum := sha256.Sum256([]byte(in.Password))
	if subtle.ConstantTimeCompare(sum[:], s.password[:]) != 1 {
		fail(w, 401, "Incorrect password")
		return
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		fail(w, 500, err)
		return
	}
	id := hex.EncodeToString(token)
	s.mu.Lock()
	for k, t := range s.sessions {
		if now.After(t) {
			delete(s.sessions, k)
		}
	}
	s.sessions[id] = now.Add(sessionLifetime)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "webui_session", Value: id, Path: "/", HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteStrictMode, MaxAge: int(sessionLifetime / time.Second)})
	respond(w, 200, true)
}

// Original file semantics are deliberately retained behind administrator authentication.
var rpcMethods = map[string]bool{}

func init() {
	for _, name := range strings.Fields("AbsolutePath CloseMMDB CopyFile Download Exec ExecBackground FileExists FileSHA256 GetEnv GetInterfaces IsStartup KillProcess MakeDir MoveFile OpenMMDB ProcessInfo ProcessMemory QueryMMDB ReadDir ReadFile RemoveFile Requests TcpPing TcpRequest UdpRequest UnzipGZFile UnzipTarGZFile UnzipZIPFile Upload WriteFile") {
		rpcMethods[name] = true
	}
}
func (s *Server) rpc(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("method")
	if name == "GetSystemProxy" || name == "GetSystemProxyBypass" {
		respond(w, 200, bridge.FlagResult{Flag: true, Data: ""})
		return
	}
	if !rpcMethods[name] {
		fail(w, 400, "Unsupported WebUI operation: "+name)
		return
	}
	var args []json.RawMessage
	if err := decode(w, r, &args); err != nil {
		fail(w, 400, err)
		return
	}
	method := reflect.ValueOf(s.app).MethodByName(name)
	typ := method.Type()
	if len(args) != typ.NumIn() {
		fail(w, 400, "Wrong argument count")
		return
	}
	values := make([]reflect.Value, len(args))
	for i, arg := range args {
		v := reflect.New(typ.In(i))
		if err := json.Unmarshal(arg, v.Interface()); err != nil {
			fail(w, 400, err)
			return
		}
		values[i] = v.Elem()
	}
	defer func() {
		if v := recover(); v != nil {
			fail(w, 500, "Bridge operation failed")
		}
	}()
	result := method.Call(values)
	if len(result) == 0 {
		respond(w, 200, nil)
	} else {
		respond(w, 200, result[0].Interface())
	}
}
func (s *Server) eventStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		fail(w, 500, "Streaming unavailable")
		return
	}
	ch, off := events.Subscribe(s.app.Ctx)
	defer off()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	_, _ = fmt.Fprint(w, "event: ready\ndata: {}\n\n")
	flusher.Flush()
	timer := time.NewTicker(15 * time.Second)
	defer timer.Stop()
	for {
		select {
		case e := <-ch:
			b, _ := json.Marshal(e)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		case <-timer.C:
			if !s.authenticated(r) {
				return
			}
			_, _ = fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		case <-s.closed:
			return
		}
	}
}

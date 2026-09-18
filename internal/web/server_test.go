package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"guiforcores/bridge"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// A real child process makes the lifecycle tests independent of installed cores.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && (os.Args[1] == "check" || os.Args[1] == "run") {
		var p string
		for i, arg := range os.Args {
			if arg == "-c" && i+1 < len(os.Args) {
				p = os.Args[i+1]
			}
		}
		b, err := os.ReadFile(p)
		if err != nil {
			os.Exit(2)
		}
		if strings.Contains(string(b), `"invalid":true`) {
			os.Exit(2)
		}
		if os.Args[1] == "check" {
			os.Exit(0)
		}
		if strings.Contains(string(b), `"crash":true`) {
			os.Exit(3)
		}
		// Keep a real child alive until the manager terminates it.
		for {
			time.Sleep(time.Second)
		}
	}
	os.Exit(m.Run())
}
func newTestServer(t *testing.T) *Server {
	t.Helper()
	app, err := bridge.CreateWebApp(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	exe, _ := os.Executable()
	s, err := New(app, fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("test frontend")}}, exe, "test-password", false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}
func request(t *testing.T, s *Server, method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.Header.Set("X-WebUI-Request", "1")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}
func login(t *testing.T, s *Server) *http.Cookie {
	t.Helper()
	w := request(t, s, "POST", "/api/login", map[string]string{"password": "test-password"}, nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	return w.Result().Cookies()[0]
}
func TestAuthenticationAndOriginalFileSemantics(t *testing.T) {
	s := newTestServer(t)
	for _, p := range []string{"/api/core/status", "/api/core/proxy/version", "/api/dashboard/dashboard/", "/api/events"} {
		if w := request(t, s, "GET", p, nil, nil); w.Code != 401 {
			t.Fatalf("%s: %d", p, w.Code)
		}
	}
	if w := request(t, s, "POST", "/api/rpc/ReadFile", []any{"/tmp/example", bridge.IOOptions{}}, nil); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := request(t, s, "POST", "/api/login", map[string]string{"password": "wrong"}, nil); w.Code != 401 {
		t.Fatal(w.Code)
	}
	cookie := login(t, s)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("cookie attributes")
	}
	// An absolute path outside the app data directory remains supported by design.
	target := filepath.Join(t.TempDir(), "plugin.txt")
	w := request(t, s, "POST", "/api/rpc/WriteFile", []any{target, "插件 UTF-8", bridge.IOOptions{Mode: "Text"}}, cookie)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = request(t, s, "POST", "/api/rpc/ReadFile", []any{target, bridge.IOOptions{Mode: "Text"}}, cookie)
	var result bridge.FlagResult
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if !result.Flag || result.Data != "插件 UTF-8" {
		t.Fatal(w.Body.String())
	}
	w = request(t, s, "POST", "/api/rpc/ReadFile", []any{target, bridge.IOOptions{Mode: "Binary"}}, cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if !result.Flag || result.Data == "插件 UTF-8" {
		t.Fatal("binary semantics")
	}
	r := httptest.NewRequest("POST", "/api/core/stop", strings.NewReader("{}"))
	r.AddCookie(cookie)
	r.Header.Set("Origin", "https://other.example")
	r.Header.Set("X-WebUI-Request", "1")
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-origin write accepted")
	}
	request(t, s, "POST", "/api/logout", map[string]any{}, cookie)
	if request(t, s, "GET", "/api/core/status", nil, cookie).Code != 401 {
		t.Fatal("logout did not revoke session")
	}
}
func TestCoreLifecycleValidationRollbackAndRestore(t *testing.T) {
	s := newTestServer(t)
	input := CoreInput{Config: json.RawMessage(`{"outbounds":[]}`), Profile: json.RawMessage(`{"id":"test"}`)}
	if err := s.core.Apply(input); err != nil {
		t.Fatal(err)
	}
	first := s.core.Status()
	if !first["running"].(bool) {
		t.Fatal(first)
	}
	// Disconnecting/expiring the browser session cannot stop the child.
	cookie := login(t, s)
	request(t, s, "POST", "/api/logout", map[string]any{}, cookie)
	if !s.core.Status()["running"].(bool) {
		t.Fatal("logout stopped core")
	}
	if err := s.core.Apply(input); err == nil {
		t.Fatal("stale revision accepted")
	}
	input.Revision = 1
	input.Config = json.RawMessage(`{"invalid":true}`)
	if err := s.core.Apply(input); err == nil {
		t.Fatal("invalid configuration accepted")
	}
	if s.core.Status()["pid"] != first["pid"] {
		t.Fatal("validation stopped healthy core")
	}
	input.Config = json.RawMessage(`{"crash":true}`)
	if err := s.core.Apply(input); err == nil {
		t.Fatal("startup failure not reported")
	}
	if !s.core.Status()["running"].(bool) || s.core.Status()["revision"] != int64(1) {
		t.Fatal("previous configuration not restored", s.core.Status())
	}
	s.core.Shutdown()
	exe, _ := os.Executable()
	restored := NewCore(s.app, exe)
	defer restored.Shutdown()
	if err := restored.Restore(); err != nil {
		t.Fatal(err)
	}
	if !restored.Status()["running"].(bool) {
		t.Fatal("service restore did not start core")
	}
	if err := restored.Stop(); err != nil {
		t.Fatal(err)
	}
	stopped := NewCore(s.app, exe)
	defer stopped.Shutdown()
	if err := stopped.Restore(); err != nil {
		t.Fatal(err)
	}
	if stopped.Status()["running"].(bool) {
		t.Fatal("explicitly stopped core restarted")
	}
}
func TestProxyUsesSavedServerAddressAndSecret(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer core-secret" || r.Header.Get("Cookie") != "" {
			t.Error("proxy credentials")
		}
		fmt.Fprint(w, r.URL.Path)
	}))
	defer upstream.Close()
	s := newTestServer(t)
	host := strings.TrimPrefix(upstream.URL, "http://")
	s.core.saved.Input.Config = json.RawMessage(fmt.Sprintf(`{"experimental":{"clash_api":{"external_controller":%q,"secret":"core-secret"}}}`, host))
	w := request(t, s, "GET", "/api/core/proxy/version", nil, login(t, s))
	if w.Code != 200 || w.Body.String() != "/version" {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestSSEAndRPCArguments(t *testing.T) {
	s := newTestServer(t)
	cookie := login(t, s)
	if w := request(t, s, "POST", "/api/rpc/WriteFile", []any{"missing-arguments"}, cookie); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w := request(t, s, "POST", "/api/rpc/StartServer", []any{}, cookie); w.Code != 400 {
		t.Fatal(w.Code)
	}
	actual := httptest.NewServer(s.Handler())
	defer actual.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, _ := http.NewRequestWithContext(ctx, "GET", actual.URL+"/api/events", nil)
	r.AddCookie(cookie)
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b := make([]byte, 25)
	n, _ := io.ReadAtLeast(res.Body, b, 14)
	if !strings.Contains(string(b[:n]), "event: ready") {
		t.Fatal(string(b[:n]))
	}
}

func TestConcurrentCoreApply(t *testing.T) {
	s := newTestServer(t)
	in := CoreInput{Config: json.RawMessage(`{"outbounds":[]}`)}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { results <- s.core.Apply(in) }()
	}
	success := 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("want exactly one successful apply, got %d", success)
	}
	if s.core.Status()["revision"] != int64(1) {
		t.Fatal(s.core.Status())
	}
}

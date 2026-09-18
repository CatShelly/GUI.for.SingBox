package web

import (
	"context"
	"encoding/json"
	"fmt"
	"guiforcores/bridge"
	"guiforcores/internal/events"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type CoreInput struct {
	Config   json.RawMessage   `json:"config"`
	Profile  json.RawMessage   `json:"profile"`
	Alpha    bool              `json:"alpha"`
	Env      map[string]string `json:"env"`
	Revision int64             `json:"revision"`
}
type savedCore struct {
	Input   CoreInput `json:"input"`
	Running bool      `json:"running"`
}
type Core struct {
	mu         sync.Mutex
	app        *bridge.App
	executable string
	dir        string
	saved      savedCore
	cmd        *exec.Cmd
	done       chan struct{}
	lastError  string
}

func NewCore(app *bridge.App, executable string) *Core {
	return &Core{app: app, executable: executable, dir: filepath.Join(bridge.Env.BasePath, "data", "sing-box")}
}
func (c *Core) path(alpha bool) string {
	if c.executable != "" {
		p, _ := filepath.Abs(c.executable)
		return p
	}
	name := "sing-box"
	if alpha {
		name += "-latest"
	}
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(c.dir, name)
}
func (c *Core) statePath() string { return filepath.Join(c.dir, "webui-state.json") }
func atomicWrite(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".webui-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func (c *Core) save() error {
	b, err := json.MarshalIndent(c.saved, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(c.statePath(), b)
}
func (c *Core) statusLocked() map[string]any {
	pid := -1
	if c.cmd != nil {
		select {
		case <-c.done:
		default:
			pid = c.cmd.Process.Pid
		}
	}
	return map[string]any{"running": pid > 0, "pid": pid, "revision": c.saved.Input.Revision, "profile": c.saved.Input.Profile, "alpha": c.saved.Input.Alpha, "error": c.lastError, "executable": c.path(c.saved.Input.Alpha)}
}
func (c *Core) Status() map[string]any { c.mu.Lock(); defer c.mu.Unlock(); return c.statusLocked() }
func (c *Core) emit()                  { events.EventsEmit(c.app.Ctx, "webui:core", c.statusLocked()) }
func (c *Core) Restore() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, err := os.ReadFile(c.statePath())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, &c.saved); err != nil {
		return err
	}
	if c.saved.Running {
		if err = c.writeConfig(c.saved.Input.Config); err != nil {
			return err
		}
		return c.startLocked(c.saved.Input)
	}
	return nil
}
func (c *Core) writeConfig(b []byte) error {
	return atomicWrite(filepath.Join(c.dir, "config.json"), b)
}
func (c *Core) Apply(in CoreInput) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if in.Revision != c.saved.Input.Revision {
		return fmt.Errorf("Core configuration changed in another session; refresh and retry")
	}
	var config map[string]any
	if err := json.Unmarshal(in.Config, &config); err != nil || config == nil {
		return fmt.Errorf("Configuration must be a JSON object")
	}
	// Validate before stopping the current core or replacing its configuration.
	candidate := filepath.Join(c.dir, ".webui-candidate.json")
	if err := atomicWrite(candidate, in.Config); err != nil {
		return err
	}
	defer os.Remove(candidate)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	check := exec.CommandContext(ctx, c.path(in.Alpha), "check", "-c", candidate, "-D", c.dir)
	check.Dir = bridge.Env.BasePath
	check.Env = environment(in.Env)
	bridge.SetCmdWindowHidden(check)
	if out, err := check.CombinedOutput(); err != nil {
		return fmt.Errorf("sing-box check failed: %v: %s", err, out)
	}
	old := c.saved
	wasRunning := c.statusLocked()["running"].(bool)
	if err := c.stopLocked(); err != nil {
		return err
	}
	in.Revision = old.Input.Revision + 1
	err := c.writeConfig(in.Config)
	if err == nil {
		err = c.startLocked(in)
	}
	if err == nil {
		c.saved = savedCore{Input: in, Running: true}
		err = c.save()
	}
	if err != nil {
		_ = c.stopLocked()
		c.saved = old
		restoreErr := error(nil)
		if len(old.Input.Config) > 0 {
			restoreErr = c.writeConfig(old.Input.Config)
			if restoreErr == nil && wasRunning {
				restoreErr = c.startLocked(old.Input)
			}
		}
		c.lastError = fmt.Sprintf("Apply failed: %v; previous configuration restore: %v", err, restoreErr)
		c.emit()
		return fmt.Errorf("%s", c.lastError)
	}
	c.lastError = ""
	c.emit()
	return nil
}
func environment(env map[string]string) []string {
	out := os.Environ()
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}
func (c *Core) startLocked(in CoreInput) error {
	logFile, err := os.OpenFile(filepath.Join(c.dir, "sing-box.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	cmd := exec.Command(c.path(in.Alpha), "run", "--disable-color", "-c", filepath.Join(c.dir, "config.json"), "-D", c.dir)
	cmd.Dir = bridge.Env.BasePath
	cmd.Env = environment(in.Env)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	bridge.SetCmdWindowHidden(cmd)
	if err = cmd.Start(); err != nil {
		logFile.Close()
		return err
	}
	c.cmd = cmd
	done := make(chan struct{})
	c.done = done
	_ = os.WriteFile(filepath.Join(c.dir, "pid.txt"), []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
	go func() {
		err := cmd.Wait()
		logFile.Close()
		close(done)
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.cmd == cmd {
			c.cmd = nil
			_ = os.Remove(filepath.Join(c.dir, "pid.txt"))
			if err != nil {
				c.lastError = err.Error()
			}
			c.emit()
		}
	}()
	target, secret := controller(in.Config, false)
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return fmt.Errorf("Core exited during startup; inspect data/sing-box/sing-box.log")
		case <-deadline.C:
			_ = c.stopLocked()
			return fmt.Errorf("Core did not become ready in 15 seconds")
		case <-ticker.C:
			if target == "" {
				return nil
			}
			req, _ := http.NewRequest("GET", "http://"+target+"/version", nil)
			req.Header.Set("Authorization", "Bearer "+secret)
			client := &http.Client{Timeout: time.Second}
			res, err := client.Do(req)
			if err == nil {
				res.Body.Close()
				if res.StatusCode == 200 {
					return nil
				}
			}
		}
	}
}
func (c *Core) stopLocked() error {
	if c.cmd == nil {
		return nil
	}
	cmd := c.cmd
	select {
	case <-c.done:
		c.cmd = nil
		return nil
	default:
	}
	_ = bridge.SendExitSignal(cmd.Process)
	select {
	case <-c.done:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		select {
		case <-c.done:
		case <-time.After(3 * time.Second):
			return fmt.Errorf("Core stop timed out")
		}
	}
	c.cmd = nil
	_ = os.Remove(filepath.Join(c.dir, "pid.txt"))
	return nil
}
func (c *Core) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.stopLocked(); err != nil {
		return err
	}
	c.saved.Running = false
	err := c.save()
	c.emit()
	return err
}
func (c *Core) Shutdown() { c.mu.Lock(); defer c.mu.Unlock(); _ = c.stopLocked() }
func (c *Core) Restart() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.saved.Input.Config) == 0 {
		return fmt.Errorf("No applied configuration")
	}
	if err := c.stopLocked(); err != nil {
		return err
	}
	if err := c.writeConfig(c.saved.Input.Config); err != nil {
		return err
	}
	if err := c.startLocked(c.saved.Input); err != nil {
		c.lastError = err.Error()
		c.emit()
		return err
	}
	c.saved.Running = true
	err := c.save()
	c.emit()
	return err
}
func controller(raw json.RawMessage, dashboard bool) (string, string) {
	var config struct {
		Experimental struct {
			ClashAPI struct {
				ExternalController string `json:"external_controller"`
				Secret             string `json:"secret"`
			} `json:"clash_api"`
		} `json:"experimental"`
		Services []struct {
			Type   string `json:"type"`
			Listen string `json:"listen"`
			Port   int    `json:"listen_port"`
			Secret string `json:"secret"`
		} `json:"services"`
	}
	if json.Unmarshal(raw, &config) != nil {
		return "", ""
	}
	addr, secret := config.Experimental.ClashAPI.ExternalController, config.Experimental.ClashAPI.Secret
	if dashboard {
		addr = ""
		for _, s := range config.Services {
			if s.Type == "api" {
				addr = net.JoinHostPort(s.Listen, strconv.Itoa(s.Port))
				secret = s.Secret
				break
			}
		}
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", ""
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port), secret
}
func (c *Core) Proxy(dashboard bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		target, secret := controller(c.saved.Input.Config, dashboard)
		c.mu.Unlock()
		if target == "" {
			fail(w, 503, "Core API is not configured")
			return
		}
		prefix := "/api/core/proxy"
		if dashboard {
			prefix = "/api/dashboard"
		}
		destination, _ := url.Parse("http://" + target)
		proxy := httputil.NewSingleHostReverseProxy(destination)
		director := proxy.Director
		proxy.Director = func(req *http.Request) {
			director(req)
			req.URL.Path = strings.TrimPrefix(req.URL.Path, prefix)
			req.URL.RawPath = ""
			req.Host = destination.Host
			req.Header.Del("Cookie")
			req.Header.Set("Authorization", "Bearer "+secret)
			q := req.URL.Query()
			q.Del("token")
			if strings.EqualFold(req.Header.Get("Upgrade"), "websocket") {
				q.Set("token", secret)
			}
			req.URL.RawQuery = q.Encode()
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			fail(w, 502, "Core API unavailable: "+err.Error())
		}
		proxy.ServeHTTP(w, r)
	}
}

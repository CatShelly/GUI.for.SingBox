package bridge

import (
	"context"
	"guiforcores/internal/events"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var Config = &AppConfig{}
var Env = &EnvResult{AppName: "singbox-webui", AppVersion: "v1.27.0-webui.1", OS: runtime.GOOS, ARCH: runtime.GOARCH}

func CreateWebApp(ctx context.Context, base string) (*App, error) {
	path, err := filepath.Abs(base)
	if err != nil {
		return nil, err
	}
	Env.BasePath = filepath.ToSlash(path)
	if err = os.MkdirAll(filepath.Join(path, "data", "sing-box"), 0755); err != nil {
		return nil, err
	}
	Env.IsPrivileged, _ = IsPrivileged()
	return &App{Ctx: events.NewContext(ctx)}, nil
}
func (a *App) GetEnv(key string) any {
	if key != "" {
		return os.Getenv(key)
	}
	return Env
}
func (a *App) IsStartup() bool { return false }
func (a *App) GetInterfaces() FlagResult {
	interfaces, err := net.Interfaces()
	if err != nil {
		return FlagResult{false, err.Error()}
	}
	names := []string{}
	for _, i := range interfaces {
		names = append(names, i.Name)
	}
	return FlagResult{true, strings.Join(names, "|")}
}

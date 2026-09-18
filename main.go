package main

import (
	"context"
	"embed"
	"flag"
	"guiforcores/bridge"
	"guiforcores/internal/web"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

//go:embed all:frontend/dist
var frontendAssets embed.FS

func embeddedFrontend() fs.FS {
	assets, err := fs.Sub(frontendAssets, "frontend/dist")
	if err != nil {
		panic(err)
	}
	return assets
}

func main() {
	listen := flag.String("listen", "0.0.0.0:9090", "HTTP listen address")
	base := flag.String("data-dir", ".", "Base directory containing data/")
	secure := flag.Bool("secure-cookie", false, "Require HTTPS session cookies, including behind a reverse proxy")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	app, err := bridge.CreateWebApp(context.Background(), *base)
	if err != nil {
		log.Fatal(err)
	}
	passwordFile := filepath.Join(bridge.Env.BasePath, "data", "user.yaml")
	password, generated, err := web.LoadPassword(passwordFile)
	if err != nil {
		log.Fatal(err)
	}
	if generated {
		log.Printf("Initial WebUI password saved in %s (webuiPassword)", passwordFile)
	}
	server, err := web.New(app, embeddedFrontend(), "", password, *secure)
	if err != nil {
		log.Fatal(err)
	}
	httpServer := &http.Server{Addr: *listen, Handler: server.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second}
	go func() {
		<-ctx.Done()
		server.Close()
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = httpServer.Shutdown(shutdown)
	}()
	log.Printf("WebUI listening on %s; data base: %s", *listen, bridge.Env.BasePath)
	if err = httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		server.Close()
		log.Fatal(err)
	}
}

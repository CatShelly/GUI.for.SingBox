package main

import (
	"context"
	"guiforcores/bridge"
	"guiforcores/internal/web"
	"io/fs"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestEmbeddedFrontendWithoutExternalFiles(t *testing.T) {
	// Serving must work from an empty working directory, without frontend/dist.
	t.Chdir(t.TempDir())
	if _, err := os.Stat("frontend/dist"); !os.IsNotExist(err) {
		t.Fatal("test directory is not isolated")
	}
	assets := embeddedFrontend()
	app, err := bridge.CreateWebApp(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server, err := web.New(app, assets, "", "test-password", false)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	handler := server.Handler()
	index := httptest.NewRecorder()
	handler.ServeHTTP(index, httptest.NewRequest("GET", "/", nil))
	if index.Code != 200 || !strings.Contains(index.Body.String(), "SingBox WebUI") {
		t.Fatalf("index: %d %s", index.Code, index.Body.String())
	}
	js, err := fs.Glob(assets, "assets/*.js")
	if err != nil || len(js) == 0 {
		t.Fatal("compiled JavaScript missing", err)
	}
	for _, name := range append(js, "favicon.ico") {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", "/"+name, nil))
		if response.Code != 200 || response.Body.Len() == 0 {
			t.Fatalf("asset %s: %d", name, response.Code)
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/api/core/status", nil))
	if response.Code != 401 {
		t.Fatal("embedding bypassed API authentication")
	}
}

package web

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"guiforcores/bridge"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Exercise the authenticated download/extract/install bridge used by core settings,
// then launch both branches from the managed directory without a core override.
func TestDownloadAndRunManagedCore(t *testing.T) {
	s := newTestServer(t)
	s.core.executable = ""
	cookie := login(t, s)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(s.core.path(false))
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "release/" + name, Size: int64(len(binary)), Mode: 0755}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(archive.Bytes()) }))
	defer remote.Close()
	downloadArgs := []any{"GET", remote.URL, "data/.cache/core.tar.gz", map[string]string{}, "", bridge.RequestOptions{Sha256: "invalid"}}
	if w := request(t, s, "POST", "/api/rpc/Download", downloadArgs, nil); w.Code != 401 {
		t.Fatal("unauthenticated download accepted")
	}
	var result bridge.HTTPResult
	w := request(t, s, "POST", "/api/rpc/Download", downloadArgs, cookie)
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Flag {
		t.Fatal("invalid checksum accepted")
	}
	downloadArgs[5] = bridge.RequestOptions{Sha256: fmt.Sprintf("%x", sha256.Sum256(archive.Bytes()))}
	w = request(t, s, "POST", "/api/rpc/Download", downloadArgs, cookie)
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || !result.Flag {
		t.Fatal(w.Body.String(), err)
	}
	rpc := func(method string, args []any) {
		t.Helper()
		w := request(t, s, "POST", "/api/rpc/"+method, args, cookie)
		var r bridge.FlagResult
		if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil || w.Code != 200 || !r.Flag {
			t.Fatal(method, w.Body.String(), err)
		}
	}
	for _, alpha := range []bool{false, true} {
		rpc("UnzipTarGZFile", []any{"data/.cache/core.tar.gz", "data/.cache"})
		rpc("MoveFile", []any{"data/.cache/release/" + name, s.core.path(alpha)})
		in := CoreInput{Alpha: alpha, Config: json.RawMessage(`{"outbounds":[]}`), Revision: s.core.saved.Input.Revision}
		if err := s.core.Apply(in); err != nil {
			t.Fatal(err)
		}
		if s.core.Status()["running"] != true {
			t.Fatal(s.core.Status())
		}
		if err := s.core.Stop(); err != nil {
			t.Fatal(err)
		}
	}
}

package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPasswordSettings(t *testing.T) {
	for _, content := range []string{"", "theme: dark\n# keep me\nkernel:\n  branch: main\n", "webuiPassword: ''\n"} {
		path := filepath.Join(t.TempDir(), "user.yaml")
		if content != "" {
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
		}
		password, generated, err := LoadPassword(path)
		if err != nil || !generated || len(password) != 48 {
			t.Fatal(generated, err)
		}
		again, generated, err := LoadPassword(path)
		if err != nil || generated || again != password {
			t.Fatal("password not persisted", err)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var settings map[string]any
		if err := yaml.Unmarshal(b, &settings); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(content, "theme:") && (settings["theme"] != "dark" || !strings.Contains(string(b), "# keep me")) {
			t.Fatal("existing settings lost")
		}
	}
	for _, content := range []string{"webuiPassword: 'my custom password'\n", "webuiPassword: [wrong]\n", "broken: [\n"} {
		path := filepath.Join(t.TempDir(), "user.yaml")
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		password, generated, err := LoadPassword(path)
		if generated {
			t.Fatal("unexpected rewrite")
		}
		if strings.Contains(content, "my custom password") {
			if err != nil || password != "my custom password" {
				t.Fatal(err)
			}
		} else if err == nil {
			t.Fatal("invalid settings accepted")
		}
		b, _ := os.ReadFile(path)
		if string(b) != content {
			t.Fatal("settings changed")
		}
	}
}

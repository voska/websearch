package auth

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestEnvironmentPrecedesFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(os.Getenv("HOME"), ".config", "voska", "ai-key")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fixture-from-file\n"), 0400); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ env, want string }{
		{"fixture-from-env", "fixture-from-env"},
		{"", "fixture-from-file"},
	} {
		t.Setenv("VOSKA_AI_KEY", tt.env)
		got, err := Key(context.Background())
		if err != nil || got != tt.want {
			t.Fatalf("wrong credential source: %v", err)
		}
	}
}

func TestFileRejectsUnsafeInputs(t *testing.T) {
	for _, tt := range []struct {
		name    string
		mode    os.FileMode
		content string
	}{
		{"group readable", 0640, "fixture"},
		{"empty", 0600, ""},
		{"multiple lines", 0600, "fixture\nfixture"},
		{"oversized", 0600, strings.Repeat("x", keyLimit+1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ai-key")
			if err := os.WriteFile(path, []byte(tt.content), tt.mode); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, tt.mode); err != nil {
				t.Fatal(err)
			}
			if _, err := readFile(path); err == nil {
				t.Fatal("accepted an unsafe file")
			}
		})
	}
}

func TestFileCanLinkToPrivateMount(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mounted")
	if err := os.WriteFile(target, []byte("fixture"), 0400); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "ai-key")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if got, err := readFile(path); err != nil || got != "fixture" {
		t.Fatalf("read private mount: %v", err)
	}
}

func TestFileRejectsNamedPipe(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ai-key")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readFile(path); err == nil {
		t.Fatal("accepted a named pipe")
	}
}

func TestBadConfiguration(t *testing.T) {
	t.Setenv("VOSKA_AI_KEY", "invalid value")
	if _, err := Key(context.Background()); err == nil {
		t.Fatal("accepted a credential with whitespace")
	}
	t.Setenv("VOSKA_AI_KEY", "")
	t.Setenv("HOME", "")
	if _, err := Key(context.Background()); err == nil {
		t.Fatal("accepted an empty home directory")
	}
}

func TestMissingKeyFile(t *testing.T) {
	if _, err := readFile(filepath.Join(t.TempDir(), "missing")); !os.IsNotExist(err) {
		t.Fatalf("err = %v, want file not found", err)
	}
	if _, err := readFile("name\x00"); err == nil {
		t.Fatal("accepted a bad path")
	}
}

func TestBrokenKeyLinkIsConfigError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ai-key")
	if err := os.Symlink(filepath.Join(dir, "missing"), path); err != nil {
		t.Fatal(err)
	}
	if _, err := readFile(path); err == nil || os.IsNotExist(err) {
		t.Fatalf("err = %v, want configuration error", err)
	}
}

package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

var ErrMissing = errors.New("no LiteLLM key found. Set VOSKA_AI_KEY, provision ~/.config/voska/ai-key, or unlock the login Keychain")

const keyLimit = 4096

func Key(ctx context.Context) (string, error) {
	if value := os.Getenv("VOSKA_AI_KEY"); strings.TrimSpace(value) != "" {
		return validate(value, "VOSKA_AI_KEY")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.New("cannot locate the home directory for ~/.config/voska/ai-key")
	}
	key, err := readFile(filepath.Join(home, ".config", "voska", "ai-key"))
	if !errors.Is(err, os.ErrNotExist) {
		return key, err
	}

	if runtime.GOOS == "darwin" {
		keyCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		data, err := exec.CommandContext(keyCtx, "/usr/bin/security", "find-generic-password",
			"-s", "org.voska.litellm.client", "-w").Output()
		if err == nil {
			return validate(string(data), "the LiteLLM login Keychain item")
		}
		if errors.Is(keyCtx.Err(), context.DeadlineExceeded) {
			return "", errors.New("login Keychain lookup timed out. Unlock it or provision ~/.config/voska/ai-key")
		}
	}
	return "", ErrMissing
}

func readFile(path string) (string, error) {
	// O_NONBLOCK prevents a misconfigured named pipe from stalling startup.
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) {
		if _, lookupErr := os.Lstat(path); !errors.Is(lookupErr, os.ErrNotExist) {
			return "", errors.New("~/.config/voska/ai-key points to an unavailable key file")
		}
		return "", err
	}
	if err != nil {
		return "", errors.New("cannot open ~/.config/voska/ai-key")
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", errors.New("cannot stat ~/.config/voska/ai-key")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || !ok || int(stat.Uid) != os.Geteuid() {
		return "", errors.New("~/.config/voska/ai-key must be a regular file owned by this user with no group or other permissions")
	}

	data, err := io.ReadAll(io.LimitReader(file, keyLimit+1))
	if err != nil {
		return "", errors.New("cannot read ~/.config/voska/ai-key")
	}
	return validate(string(data), "~/.config/voska/ai-key")
}

func validate(value, source string) (string, error) {
	key := strings.TrimSpace(value)
	if len(value) > keyLimit || key == "" || strings.ContainsFunc(key, func(r rune) bool { return r < '!' || r > '~' }) {
		return "", fmt.Errorf("%s must contain one nonempty LiteLLM key", source)
	}
	return key, nil
}

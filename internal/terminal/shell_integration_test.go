package terminal

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	xterm "github.com/eugenioenko/xterm-go"
)

func useTempCache(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("HOME", dir)
}

func TestShellArgsLoadsBashIntegration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash integration is Unix-only")
	}
	useTempCache(t)

	args := shellArgs("/usr/bin/bash")
	if len(args) != 2 || args[0] != "--rcfile" {
		t.Fatalf("shellArgs(bash) = %q, want --rcfile <path>", args)
	}
	got, err := os.ReadFile(args[1])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, bashIntegration) {
		t.Fatal("written integration script differs from the embedded one")
	}

	if info, err := os.Stat(args[1]); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("integration script mode = %v, want 0600", info.Mode().Perm())
	}

	if err := os.WriteFile(args[1], []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	shellArgs("/usr/bin/bash")
	if got, _ := os.ReadFile(args[1]); !bytes.Equal(got, bashIntegration) {
		t.Fatal("a stale integration script was not replaced")
	}
}

// Another user able to write the directory could replace the script bash
// runs, so a loosened directory is tightened back to private.
func TestShellArgsKeepsScriptDirectoryPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash integration is Unix-only")
	}
	useTempCache(t)
	dir := filepath.Join(os.Getenv("XDG_CACHE_HOME"), "ttt")
	if err := os.MkdirAll(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}

	if shellArgs("/bin/bash") == nil {
		t.Fatal("shellArgs gave up on a directory the user owns")
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("script directory mode = %v, want 0700", info.Mode().Perm())
	}
}

func TestShellArgsLeavesOtherShellsAlone(t *testing.T) {
	useTempCache(t)
	for _, shell := range []string{"/bin/sh", "/usr/bin/fish", "/bin/zsh", "bashful"} {
		if args := shellArgs(shell); args != nil {
			t.Errorf("shellArgs(%q) = %q, want nil", shell, args)
		}
	}
}

func TestBashIntegrationMarksPromptAndCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash integration is Unix-only")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not installed")
	}
	useTempCache(t)

	term, err := New(bash, 80, 24, 0, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	term.Run()
	defer term.Close()

	waitForRaw(t, term, "\x1b]133;A;redraw=last\a")
	term.WriteString("true\r")
	waitForRaw(t, term, "\x1b]133;C\a")
}

func waitForRaw(t *testing.T, term *Terminal, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(string(term.RawTail()), want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("shell never sent %q; got %q", want, term.RawTail())
}

func TestBashPromptSurvivesResize(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash integration is Unix-only")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not installed")
	}
	useTempCache(t)
	prompt := "long-prompt-" + strings.Repeat("p", 40) + " $ "
	if err := os.WriteFile(os.Getenv("HOME")+"/.bashrc", []byte("PS1='"+prompt+"'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	term, err := New(bash, 80, 10, 0, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	term.Run()
	defer term.Close()

	marks := 1
	waitForRawCount(t, term, "\x1b]133;A", marks)
	for _, cols := range []int{40, 30, 45, 80} {
		term.Resize(cols, 10)
		marks++
		waitForRawCount(t, term, "\x1b]133;A", marks)
	}

	var screen string
	term.Snapshot(func(x *xterm.Terminal) { screen = x.String() })
	if n := strings.Count(screen, "long-prompt-"); n != 1 {
		t.Fatalf("%d prompts after resizing, want 1:\n%s", n, screen)
	}
}

func waitForRawCount(t *testing.T, term *Terminal, want string, count int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Count(string(term.RawTail()), want) >= count {
			time.Sleep(50 * time.Millisecond)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("shell sent %q fewer than %d times; got %q", want, count, term.RawTail())
}

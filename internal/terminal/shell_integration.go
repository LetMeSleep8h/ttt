package terminal

import (
	"bytes"
	_ "embed"
	"os"
	"path/filepath"
	"runtime"
)

//go:embed shell/bash-integration.sh
var bashIntegration []byte

// shellArgs loads ttt's prompt marks into bash. Other shells, or a cache that
// cannot be written, start the shell as usual.
func shellArgs(shell string) []string {
	if runtime.GOOS == "windows" || filepath.Base(shell) != "bash" {
		return nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil
	}
	// bash runs this script as the user, so it lives where no one else can
	// replace it; chmod fails on a directory another user created.
	dir = filepath.Join(dir, "ttt")
	if os.MkdirAll(dir, 0o700) != nil || os.Chmod(dir, 0o700) != nil {
		return nil
	}
	path := filepath.Join(dir, "bash-integration.sh")
	if old, err := os.ReadFile(path); err != nil || !bytes.Equal(old, bashIntegration) {
		if os.WriteFile(path, bashIntegration, 0o600) != nil {
			return nil
		}
	}
	return []string{"--rcfile", path}
}

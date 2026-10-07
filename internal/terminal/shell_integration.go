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
	path := filepath.Join(dir, "ttt", "bash-integration.sh")
	if old, err := os.ReadFile(path); err != nil || !bytes.Equal(old, bashIntegration) {
		if os.MkdirAll(filepath.Dir(path), 0o755) != nil || os.WriteFile(path, bashIntegration, 0o644) != nil {
			return nil
		}
	}
	return []string{"--rcfile", path}
}

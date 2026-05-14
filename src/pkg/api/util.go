package api

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	goruntime "runtime"
	"syscall"
	"time"

	"github.com/muxi-ai/server/pkg/config"
)

// renameFn is the rename primitive used by safeRename. Defined as a
// package-level variable so the EXDEV-fallback test can inject a stub
// that returns *os.LinkError{Err: syscall.EXDEV} without needing two
// physical filesystems available in the test environment.
var renameFn = os.Rename

// resolveHealthTimeout returns the configured total timeout for the
// staging-health-check loop used by every spawn-and-wait handler
// (deploy/update/restart/start/dev/rollback). Previously deploy and
// update honored Formations.Deployment.HealthCheck.Timeout while the
// other four handlers hardcoded 300s — meaning operators who shortened
// the default 30s timeout in config.yaml got their setting silently
// ignored on four of the six code paths, and tests that legitimately
// expected fast failure (crashed python stub, no app.py) hung for
// minutes in CI. One helper keeps all six in sync.
//
// Falls back to 300s only when the config field is zero — matches the
// "5 minutes" comment already present in deploy.go/update.go, preserving
// production behavior for operators who never touched the field.
func resolveHealthTimeout(cfg *config.Config) time.Duration {
	if cfg != nil && cfg.Formations.Deployment.HealthCheck.Timeout > 0 {
		return time.Duration(cfg.Formations.Deployment.HealthCheck.Timeout) * time.Second
	}
	return 300 * time.Second
}

// isRunningInContainer checks if we're running inside a container
func isRunningInContainer() bool {
	// Check for Docker
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	// Check for container environment variable (Podman, etc.)
	if os.Getenv("container") != "" {
		return true
	}
	return false
}

// getBindHost returns the appropriate bind host for formations.
// Uses 0.0.0.0 for macOS/Windows (Docker runtime) and containers (network namespace isolation).
// Uses configured bind host (127.0.0.1) for native Linux for security.
func getBindHost(configuredHost string) string {
	if goruntime.GOOS == "darwin" || goruntime.GOOS == "windows" || isRunningInContainer() {
		return "0.0.0.0"
	}
	return configuredHost
}

// getServerTmpDir returns <DataDir>/tmp, the server's local-FS scratch
// directory for bundle extraction prior to the rename into
// formations/<id>/{current,staging}.
//
// Why this exists: the previous default of os.MkdirTemp("", ...) writes
// to $TMPDIR (usually /tmp), which on modern Linux distros is a tmpfs
// mount living on a different filesystem than /var/lib/muxi. os.Rename
// across filesystems fails with EXDEV ("invalid cross-device link"),
// which the deploy/update handlers surface as the confusing "Failed to
// move source to staging" — an error message that hides the real OS
// error code from operators and burned hours of debugging time on the
// 0.20260514.0 deploy that triggered this fix.
//
// EnsureDirectories already creates <DataDir>/tmp at every server start;
// the MkdirAll here is defensive against operators who remove the dir
// out from under a running server.
func getServerTmpDir() (string, error) {
	dataDir, err := config.GetDataDir()
	if err != nil {
		return "", fmt.Errorf("resolve data dir: %w", err)
	}
	tmp := filepath.Join(dataDir, "tmp")
	if err := os.MkdirAll(tmp, 0755); err != nil {
		return "", fmt.Errorf("create server tmp dir %q: %w", tmp, err)
	}
	return tmp, nil
}

// safeRename moves oldpath to newpath. Falls back to a recursive copy +
// remove when os.Rename returns EXDEV (cross-device link), which happens
// when source and destination live on different filesystems — e.g., the
// extract dir lands on tmpfs while the data dir is on disk.
//
// Defense in depth: callers already prefer getServerTmpDir() for extract
// dirs so the rename stays same-FS in the common path. This fallback
// exists for the edge case where MUXI_DATA_DIR is bind-mounted onto a
// different mount than the rest of /var, or where MUXI_CACHE_DIR /
// $TMPDIR overrides put things on disjoint filesystems.
//
// Non-EXDEV errors are returned as-is so the caller's logger records the
// real failure rather than the EXDEV fallback masking it.
func safeRename(oldpath, newpath string) error {
	if err := renameFn(oldpath, newpath); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return err
	}

	if err := copyTreePreservingMode(oldpath, newpath); err != nil {
		return fmt.Errorf("cross-device fallback: copy %s -> %s: %w", oldpath, newpath, err)
	}
	// Best-effort source removal — the move logically happened the moment
	// the copy finished, so we don't fail the caller if cleanup misses
	// (e.g., a stray file the user opened by hand during the deploy).
	// A leftover source under <DataDir>/tmp gets cleaned by the deferred
	// os.RemoveAll(extractDir) in the caller anyway.
	_ = os.RemoveAll(oldpath)
	return nil
}

// copyTreePreservingMode recursively copies src tree to dst, preserving
// per-file modes (notably 0600 for secrets.enc and friends) and symlinks.
//
// Distinct from the simpler copyDir in draft.go which defaults files to
// 0644 and dirs to 0755 — fine for draft workspaces but a security
// regression here, since safeRename's EXDEV fallback handles real
// deploy bundles where dropping a 0600 secrets.enc to 0644 would
// expose it to any user on the host.
func copyTreePreservingMode(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		info, err := d.Info()
		if err != nil {
			return err
		}

		switch {
		case d.IsDir():
			return os.MkdirAll(target, info.Mode().Perm())
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		default:
			return copyRegularFile(path, target, info.Mode().Perm())
		}
	})
}

func copyRegularFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// O_EXCL guards against accidental overwrite. safeRename's contract is
	// "move into a non-existent newpath"; an existing destination signals
	// a caller bug (forgot to RemoveAll first), and silently overwriting
	// would mask it.
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

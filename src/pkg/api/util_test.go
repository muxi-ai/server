package api

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestSafeRename_SameFS verifies the happy path: when os.Rename succeeds
// (source and dest on the same filesystem), safeRename returns nil and
// the original tree has been atomically moved without invoking the
// copy-and-remove fallback.
func TestSafeRename_SameFS(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "f.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("setup write: %v", err)
	}

	if err := safeRename(src, dst); err != nil {
		t.Fatalf("safeRename: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source should be gone after rename, stat err: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "f.txt"))
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("dst content = %q, want %q", got, "hello")
	}
}

// TestSafeRename_CrossFSFallback simulates EXDEV by stubbing renameFn.
// Verifies that safeRename copies the tree via the EXDEV fallback path,
// preserves per-file permissions (notably 0600 for secrets-style files),
// and cleans up the source. Stubbing avoids requiring two physical
// filesystems in the test environment.
func TestSafeRename_CrossFSFallback(t *testing.T) {
	saved := renameFn
	defer func() { renameFn = saved }()
	renameFn = func(_, _ string) error {
		return &os.LinkError{Op: "rename", Err: syscall.EXDEV}
	}

	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	subDir := filepath.Join(src, "sub")
	if err := os.MkdirAll(subDir, 0700); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "f.txt"), []byte("hi"), 0644); err != nil {
		t.Fatalf("setup write f.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "secrets.enc"), []byte("x"), 0600); err != nil {
		t.Fatalf("setup write secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("n"), 0644); err != nil {
		t.Fatalf("setup write nested: %v", err)
	}

	if err := safeRename(src, dst); err != nil {
		t.Fatalf("safeRename EXDEV fallback: %v", err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source should be cleaned up, stat err: %v", err)
	}

	// Content preserved.
	if b, _ := os.ReadFile(filepath.Join(dst, "f.txt")); string(b) != "hi" {
		t.Errorf("dst/f.txt content = %q, want %q", b, "hi")
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "sub", "nested.txt")); string(b) != "n" {
		t.Errorf("dst/sub/nested.txt content = %q, want %q", b, "n")
	}

	// 0600 mode preserved on the secrets-style file. This is the core
	// reason copyTreePreservingMode exists instead of reusing draft.go's
	// copyDir, which would silently widen the file to 0644.
	info, err := os.Stat(filepath.Join(dst, "secrets.enc"))
	if err != nil {
		t.Fatalf("stat dst/secrets.enc: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Errorf("secrets.enc mode = %v, want 0600", got)
	}

	// 0700 mode preserved on a sub-directory.
	subInfo, err := os.Stat(filepath.Join(dst, "sub"))
	if err != nil {
		t.Fatalf("stat dst/sub: %v", err)
	}
	if got := subInfo.Mode().Perm(); got != 0700 {
		t.Errorf("sub mode = %v, want 0700", got)
	}
}

// TestSafeRename_NonEXDEVErrorPropagates verifies the non-EXDEV branch:
// rename errors that aren't cross-device must bubble up verbatim so the
// caller's logger captures the real OS code instead of the
// fallback masking it as a copy failure.
func TestSafeRename_NonEXDEVErrorPropagates(t *testing.T) {
	saved := renameFn
	defer func() { renameFn = saved }()
	custom := errors.New("synthetic permission denied")
	renameFn = func(_, _ string) error { return custom }

	err := safeRename("/no/source", "/no/destination")
	if !errors.Is(err, custom) {
		t.Errorf("expected custom error to propagate, got %v", err)
	}
}

// TestGetServerTmpDir_HonorsDataDirOverride verifies that MUXI_DATA_DIR
// flows through to the resolved tmp dir and the directory is created.
// Same-FS guarantee for the rename depends on this path landing under
// the data dir, so a regression here would silently reintroduce the
// EXDEV bug on tmpfs-/tmp distros.
func TestGetServerTmpDir_HonorsDataDirOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MUXI_DATA_DIR", tmp)

	got, err := getServerTmpDir()
	if err != nil {
		t.Fatalf("getServerTmpDir: %v", err)
	}
	want := filepath.Join(tmp, "tmp")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	info, err := os.Stat(got)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected directory, got mode %v", info.Mode())
	}
}

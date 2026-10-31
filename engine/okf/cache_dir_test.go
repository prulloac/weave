package okf

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultCacheDirUsesUserCacheDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	dir, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "weave", "okf")
	if got := defaultCacheDir(); got != want {
		t.Fatalf("default cache dir = %q, want %q", got, want)
	}
}

func TestDefaultCacheDirHonorsXDGOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG semantics are Linux-specific")
	}
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	want := filepath.Join(cache, "weave", "okf")
	if got := defaultCacheDir(); got != want {
		t.Fatalf("default cache dir = %q, want %q", got, want)
	}
}

func TestDefaultCacheDirIgnoresRelativeXDGOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG semantics are Linux-specific")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", "relative-cache")
	want := filepath.Join(home, ".cache", "weave", "okf")
	if got := defaultCacheDir(); got != want {
		t.Fatalf("default cache dir = %q, want %q", got, want)
	}
}

package proc

import (
	"os"
	"testing"
)

func TestOwnProcessIsRunning(t *testing.T) {
	n, ok := Name(os.Getpid())
	if !ok || n == "" {
		t.Fatalf("own process not found: %q %v", n, ok)
	}
}

func TestInvalidPid(t *testing.T) {
	if _, ok := Name(0); ok {
		t.Fatal("pid 0 reported as running")
	}
	if _, ok := Name(-5); ok {
		t.Fatal("negative pid reported as running")
	}
}

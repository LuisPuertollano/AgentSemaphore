package proc

import (
	"os"
	"regexp"
	"runtime"
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

func TestStartTicksAndDomain(t *testing.T) {
	if _, ok := StartTicks(0); ok {
		t.Fatal("pid 0 has a start time")
	}
	ticks, ok := StartTicks(os.Getpid())
	d := Domain()
	if runtime.GOOS != "linux" {
		if ok || d != "" {
			t.Fatalf("only Linux knows the format: %v %q", ok, d)
		}
		return
	}
	if !ok {
		t.Fatal("own start time not found")
	}
	_ = ticks // 0 is possible only for very early processes; any value is fine
	if !regexp.MustCompile(`^linux:[0-9a-f]+:pid:\[\d+\]$`).MatchString(d) && d != "" {
		t.Fatalf("domain %q", d)
	}
}

package single

import "testing"

func TestSecondAcquireFailsUntilReleased(t *testing.T) {
	release, ok, err := Acquire()
	if err != nil || !ok {
		t.Fatalf("first acquire: ok=%v err=%v", ok, err)
	}
	if _, ok2, err := Acquire(); err != nil || ok2 {
		t.Fatalf("second acquire should fail: ok=%v err=%v", ok2, err)
	}
	release()
	release2, ok3, err := Acquire()
	if err != nil || !ok3 {
		t.Fatalf("acquire after release: ok=%v err=%v", ok3, err)
	}
	release2()
}

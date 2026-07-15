package session

import "testing"

func TestClampCrossfadeSec(t *testing.T) {
	if got := ClampCrossfadeSec(0); got != DefaultCrossfadeSec {
		t.Fatalf("0 => %d, want %d", got, DefaultCrossfadeSec)
	}
	if got := ClampCrossfadeSec(3); got != 3 {
		t.Fatalf("3 => %d", got)
	}
	if got := ClampCrossfadeSec(99); got != DefaultCrossfadeSec {
		t.Fatalf("99 => %d, want default", got)
	}
	if got := ClampCrossfadeSec(1); got != 1 {
		t.Fatalf("1 => %d", got)
	}
}

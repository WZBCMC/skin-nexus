package store

import (
	"errors"
	"testing"
)

// TestIsDupKeyNil guards the B2 regression: UpdateUserEmail/UpdateUserNickname
// probe isDupKey on EVERY result including success (err == nil) — the old
// implementation dereferenced err.Error() and panicked on the happy path,
// taking down the CI integration suite.
func TestIsDupKeyNil(t *testing.T) {
	if isDupKey(nil) {
		t.Fatal("isDupKey(nil) = true, want false")
	}
	if isDupKey(errors.New("boom")) {
		t.Fatal("non-duplicate error misclassified as duplicate")
	}
	if !isDupKey(errors.New("Error 1062 (23000): Duplicate entry 'x' for key 'uq'")) {
		t.Fatal("ER1062 text not classified as duplicate")
	}
}

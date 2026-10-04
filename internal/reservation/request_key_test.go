package reservation

import (
	"testing"

	"github.com/google/uuid"
)

func TestRequestKeyStableRegardlessOfSeatOrder(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	a := RequestKey(id, "user1", []string{"B1", "A1"})
	b := RequestKey(id, "user1", []string{"A1", "B1"})
	if a != b {
		t.Fatalf("keys differ: %q vs %q", a, b)
	}
}

package show

import (
	"testing"
)

func TestNormalizeSeatCodes(t *testing.T) {
	codes, err := normalizeSeatCodes([]string{"A1", " A2 "})
	if err != nil || len(codes) != 2 || codes[0] != "A1" || codes[1] != "A2" {
		t.Fatalf("unexpected: %v %v", codes, err)
	}

	_, err = normalizeSeatCodes([]string{"A1", "A1"})
	if err == nil {
		t.Fatal("expected duplicate error")
	}

	_, err = normalizeSeatCodes(nil)
	if err == nil {
		t.Fatal("expected empty seats error")
	}
}

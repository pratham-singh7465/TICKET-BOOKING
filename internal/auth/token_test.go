package auth

import "testing"

func TestParseBearerToken(t *testing.T) {
	tok, ok := ParseBearerToken("Bearer token-user1")
	if !ok || tok != "token-user1" {
		t.Fatalf("unexpected: ok=%v tok=%q", ok, tok)
	}
	if _, ok := ParseBearerToken("Basic x"); ok {
		t.Fatal("expected false for non-bearer")
	}
}

func TestHashToken_matchesPostgresDigest(t *testing.T) {
	// encode(digest('token-user1', 'sha256'), 'hex') in Postgres.
	const expected = "58ca65af512d362fb89906d92ab5f46a08f8794cabdae0d2fc0d00a61e462552"
	if HashToken("token-user1") != expected {
		t.Fatalf("hash mismatch")
	}
}

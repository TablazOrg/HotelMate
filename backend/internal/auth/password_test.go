package auth

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !VerifyPassword(hash, "correct-horse-battery-staple") {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword(hash, "incorrect-password") {
		t.Fatal("unexpected password verification")
	}
}

func TestIdentityNormalizationAndVerify(t *testing.T) {
	hash, err := HashIdentity("AB-12 34")
	if err != nil {
		t.Fatalf("hash identity: %v", err)
	}
	if !VerifyIdentity(hash, "ab1234") {
		t.Fatal("expected normalized identity to verify")
	}
}

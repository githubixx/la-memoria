package usecase

import (
	"context"
	"strings"
	"testing"
)

func TestGeneratePasswordHashProducesFreshVerifiableArgon2idPHC(t *testing.T) {
	first, err := GeneratePasswordHash("correct horse battery staple")
	if err != nil {
		t.Fatalf("generate first password hash: %v", err)
	}
	second, err := GeneratePasswordHash("correct horse battery staple")
	if err != nil {
		t.Fatalf("generate second password hash: %v", err)
	}
	if first == second {
		t.Fatal("generated password hashes must use distinct salts")
	}
	if !strings.HasPrefix(first, "$argon2id$v=19$m=65536,t=3,p=1$") {
		t.Fatalf("password hash = %q, want supported Argon2id PHC parameters", first)
	}
	verified, err := VerifyPassword(context.Background(), first, "correct horse battery staple")
	if err != nil {
		t.Fatalf("verify generated password hash: %v", err)
	}
	if !verified {
		t.Fatal("generated password hash did not verify its source password")
	}
}

func TestGeneratePasswordHashRejectsEmptyPassword(t *testing.T) {
	if _, err := GeneratePasswordHash(""); err == nil {
		t.Fatal("expected empty password to be rejected")
	}
}

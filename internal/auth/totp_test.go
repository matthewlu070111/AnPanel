package auth

import (
	"strings"
	"testing"
	"time"
)

func TestTOTP(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	now := time.Unix(1_700_000_000, 0)
	code := totpCode(secret, now)
	if len(code) != 6 || strings.Contains(code, " ") {
		t.Fatalf("invalid code %q", code)
	}
	if !VerifyTOTP(secret, code, now) {
		t.Fatal("generated code was rejected")
	}
	if VerifyTOTP(secret, "000000", now) && code != "000000" {
		t.Fatal("invalid code accepted")
	}
}

func TestNewSecret(t *testing.T) {
	a, _ := NewTOTPSecret()
	b, _ := NewTOTPSecret()
	if len(a) < 20 || a == b {
		t.Fatal("secrets must be long and unique")
	}
}

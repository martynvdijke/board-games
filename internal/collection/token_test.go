package collection

import (
	"strings"
	"testing"
)

func TestTokenSealRoundTrip(t *testing.T) {
	sealed, err := sealToken("test-session-secret", "super-secret-token")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, "super-secret-token") {
		t.Fatal("sealed value contains the plaintext token")
	}
	if !strings.HasPrefix(sealed, tokenCipherPrefix) {
		t.Fatalf("sealed value lacks version prefix: %q", sealed)
	}
	plain, err := openToken("test-session-secret", sealed)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "super-secret-token" {
		t.Fatalf("round trip = %q", plain)
	}
	again, err := sealToken("test-session-secret", "super-secret-token")
	if err != nil {
		t.Fatal(err)
	}
	if again == sealed {
		t.Fatal("sealing the same token twice produced identical ciphertext")
	}
}

func TestTokenOpenRejectsWrongKeyAndTampering(t *testing.T) {
	sealed, err := sealToken("test-session-secret", "super-secret-token")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openToken("a-different-secret", sealed); err == nil {
		t.Fatal("expected decryption with the wrong secret to fail")
	}
	tampered := sealed[:len(sealed)-2] + "AA"
	if tampered == sealed {
		tampered = sealed[:len(sealed)-2] + "BB"
	}
	if _, err := openToken("test-session-secret", tampered); err == nil {
		t.Fatal("expected tampered ciphertext to fail")
	}
	if _, err := openToken("test-session-secret", tokenCipherPrefix+"not-base64!!"); err == nil {
		t.Fatal("expected invalid base64 to fail")
	}
}

func TestTokenOpenKeepsLegacyPlaintext(t *testing.T) {
	plain, err := openToken("test-session-secret", "legacy-plaintext-token")
	if err != nil {
		t.Fatal(err)
	}
	if plain != "legacy-plaintext-token" {
		t.Fatalf("legacy value = %q", plain)
	}
}

func TestTokenSealRequiresSecret(t *testing.T) {
	if _, err := sealToken("", "super-secret-token"); err == nil {
		t.Fatal("expected sealing without a session secret to fail")
	}
}

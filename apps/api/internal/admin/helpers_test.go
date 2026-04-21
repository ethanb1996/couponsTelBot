package admin

import (
	"crypto/aes"
	"crypto/cipher"
	"strings"
	"testing"
)

func TestParseCouponCSVSupportsTwoAndThreeColumnRows(t *testing.T) {
	t.Parallel()

	input := strings.NewReader("masked-1234,secret-code,2026-12-31\nplain-code,2026-11-30\n")

	rows, err := parseCouponCSV(input)
	if err != nil {
		t.Fatalf("expected parse to succeed: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].MaskedDisplay != "masked-1234" || rows[0].PlainCode != "secret-code" {
		t.Fatalf("unexpected first row: %+v", rows[0])
	}
	if rows[1].MaskedDisplay != "***code" || rows[1].PlainCode != "plain-code" {
		t.Fatalf("unexpected second row: %+v", rows[1])
	}
}

func TestEncryptCouponCodeRoundTrip(t *testing.T) {
	t.Parallel()

	key := strings.Repeat("a", 32)
	ciphertext, nonce, err := encryptCouponCode(key, "coupon-secret")
	if err != nil {
		t.Fatalf("expected encryption to succeed: %v", err)
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		t.Fatalf("expected cipher init to succeed: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("expected gcm init to succeed: %v", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		t.Fatalf("expected decryption to succeed: %v", err)
	}

	if string(plaintext) != "coupon-secret" {
		t.Fatalf("expected plaintext to round-trip, got %q", plaintext)
	}
}

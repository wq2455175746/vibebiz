package secret

import (
	"encoding/base64"
	"strings"
	"testing"
)

func testKey() string {
	return base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	cipher, err := NewCipher(testKey())
	if err != nil {
		t.Fatal(err)
	}
	first, err := cipher.Encrypt("sk-test")
	if err != nil {
		t.Fatal(err)
	}
	second, err := cipher.Encrypt("sk-test")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("nonce should make ciphertext differ")
	}
	plain, err := cipher.Decrypt(first)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "sk-test" {
		t.Fatalf("plain = %q", plain)
	}
}

func TestNewCipherRejectsInvalidKey(t *testing.T) {
	if _, err := NewCipher(""); err == nil {
		t.Fatal("empty key should fail")
	}
	if _, err := NewCipher(base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("short key should fail")
	}
}

func TestDecryptRejectsTamperedCiphertext(t *testing.T) {
	cipher, _ := NewCipher(testKey())
	encrypted, _ := cipher.Encrypt("sk-test")
	if _, err := cipher.Decrypt(encrypted + strings.Repeat("A", 4)); err == nil {
		t.Fatal("tampered ciphertext should fail")
	}
}

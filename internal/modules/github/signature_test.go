package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestSignature(t *testing.T) {
	m := hmac.New(sha256.New, []byte("s"))
	m.Write([]byte("x"))
	sig := "sha256=" + hex.EncodeToString(m.Sum(nil))
	if !ValidSignature("s", "x", sig) || ValidSignature("s", "x", "sha256=no") {
		t.Fatal("signature validation")
	}
}

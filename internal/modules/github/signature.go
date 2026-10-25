package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func ValidSignature(secret, body, signature string) bool {
	m := hmac.New(sha256.New, []byte(secret))
	_, _ = m.Write([]byte(body))
	return hmac.Equal([]byte("sha256="+hex.EncodeToString(m.Sum(nil))), []byte(signature))
}

package components

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

type ID struct{ Version, Action, Guild, Resource, User string }

func Sign(secret string, v ID) string {
	p := strings.Join([]string{v.Version, v.Action, v.Guild, v.Resource, v.User}, "|")
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(p))
	return base64.RawURLEncoding.EncodeToString([]byte(p + "|" + base64.RawURLEncoding.EncodeToString(m.Sum(nil))))
}
func Parse(secret, raw string) (ID, error) {
	b, e := base64.RawURLEncoding.DecodeString(raw)
	if e != nil {
		return ID{}, e
	}
	x := strings.Split(string(b), "|")
	if len(x) != 6 {
		return ID{}, fmt.Errorf("invalid component")
	}
	if !hmac.Equal([]byte(Sign(secret, ID{x[0], x[1], x[2], x[3], x[4]})), []byte(raw)) {
		return ID{}, fmt.Errorf("invalid signature")
	}
	return ID{x[0], x[1], x[2], x[3], x[4]}, nil
}

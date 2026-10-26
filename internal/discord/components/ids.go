package components

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
)

// Discord custom_id is limited to 100 characters. Compact binary IDs retain
// an 80-bit HMAC while safely carrying guild, resource, and user identifiers.
type ID struct{ Version, Action, Guild, Resource, User string }

var actions = map[string]byte{"form_start": 1, "form_submit": 2, "form_approve": 3, "form_reject": 4, "form_reject_submit": 5, "ticket_open": 6, "role_toggle": 7, "rules_show": 8, "form_create": 9, "setup_member_pick": 10, "setup_pending_pick": 11, "role_pick": 12, "setup_member_set": 13, "setup_pending_set": 14, "setup_review_pick": 15, "setup_review_set": 16, "role_publish_set": 17, "panel_ticket": 18, "feed_add_modal": 19, "release_add_modal": 20, "workflow_add_modal": 21, "feed_add_submit": 22, "release_add_submit": 23, "workflow_add_submit": 24}
var names = func() map[byte]string {
	r := map[byte]string{}
	for n, c := range actions {
		r[c] = n
	}
	return r
}()

func Sign(secret string, v ID) string {
	a, ok := actions[v.Action]
	if !ok {
		panic("unknown component action: " + v.Action)
	}
	p := []byte{1, a}
	p = appendID(p, v.Guild)
	p = appendResource(p, v.Resource)
	p = appendID(p, v.User)
	m := hmac.New(sha256.New, []byte(secret))
	_, _ = m.Write(p)
	p = append(p, m.Sum(nil)[:10]...)
	return base64.RawURLEncoding.EncodeToString(p)
}
func Parse(secret, raw string) (ID, error) {
	b, e := base64.RawURLEncoding.DecodeString(raw)
	if e != nil || len(b) < 15 {
		return ID{}, fmt.Errorf("invalid component")
	}
	a, ok := names[b[1]]
	if b[0] != 1 || !ok {
		return ID{}, fmt.Errorf("invalid component")
	}
	p, tag := b[:len(b)-10], b[len(b)-10:]
	m := hmac.New(sha256.New, []byte(secret))
	_, _ = m.Write(p)
	if !hmac.Equal(tag, m.Sum(nil)[:10]) {
		return ID{}, fmt.Errorf("invalid component signature")
	}
	o := 2
	g, n, e := readID(p[o:])
	if e != nil {
		return ID{}, e
	}
	o += n
	r, n, e := readResource(p[o:])
	if e != nil {
		return ID{}, e
	}
	o += n
	u, n, e := readID(p[o:])
	if e != nil || o+n != len(p) {
		return ID{}, fmt.Errorf("invalid component")
	}
	return ID{"v1", a, g, r, u}, nil
}
func appendID(p []byte, v string) []byte {
	if v == "" {
		return append(p, 0)
	}
	n, e := strconv.ParseUint(v, 10, 64)
	if e != nil {
		if len(v) > 32 {
			panic("invalid Discord ID")
		}
		return append(append(append(p, 2), byte(len(v))), []byte(v)...)
	}
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], n)
	return append(append(p, 1), b[:]...)
}
func appendResource(p []byte, v string) []byte {
	if v == "" {
		return append(p, 0)
	}
	if n, e := strconv.ParseUint(v, 10, 64); e == nil {
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], n)
		return append(append(p, 1), b[:]...)
	}
	b, e := hex.DecodeString(v)
	if e != nil || len(b) != 16 {
		if len(v) > 32 {
			panic("invalid component resource")
		}
		return append(append(append(p, 3), byte(len(v))), []byte(v)...)
	}
	return append(append(p, 2), b...)
}
func readID(b []byte) (string, int, error) {
	if len(b) < 1 {
		return "", 0, fmt.Errorf("invalid component")
	}
	if b[0] == 0 {
		return "", 1, nil
	}
	if b[0] == 2 && len(b) >= 2 && len(b) >= 2+int(b[1]) {
		return string(b[2 : 2+int(b[1])]), 2 + int(b[1]), nil
	}
	if b[0] != 1 || len(b) < 9 {
		return "", 0, fmt.Errorf("invalid component")
	}
	return strconv.FormatUint(binary.BigEndian.Uint64(b[1:9]), 10), 9, nil
}
func readResource(b []byte) (string, int, error) {
	if len(b) < 1 {
		return "", 0, fmt.Errorf("invalid component")
	}
	if b[0] == 0 {
		return "", 1, nil
	}
	if b[0] == 1 && len(b) >= 9 {
		return strconv.FormatUint(binary.BigEndian.Uint64(b[1:9]), 10), 9, nil
	}
	if b[0] == 2 && len(b) >= 17 {
		return hex.EncodeToString(b[1:17]), 17, nil
	}
	if b[0] == 3 && len(b) >= 2 && len(b) >= 2+int(b[1]) {
		return string(b[2 : 2+int(b[1])]), 2 + int(b[1]), nil
	}
	return "", 0, fmt.Errorf("invalid component")
}

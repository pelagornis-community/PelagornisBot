package security

import (
	"context"
	"testing"
)

func TestSafeURLRejectsPrivate(t *testing.T) {
	if _, e := SafeURL(context.Background(), "http://127.0.0.1/x"); e == nil {
		t.Fatal("loopback accepted")
	}
}

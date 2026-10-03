package annette

import (
	"net/http"
	"testing"
)

func TestHeaderGet(t *testing.T) {
	h := &Header{hdr: http.Header{}}
	h.hdr.Set("Content-Type", "text/plain")
	h.hdr.Add("X-Multi", "a")
	h.hdr.Add("X-Multi", "b")

	tests := []struct {
		key  string
		want string
	}{
		{"Content-Type", "text/plain"},
		{"content-type", "text/plain"}, // case-insensitive
		{"X-Multi", "a"},               // first value
		{"X-Missing", ""},
	}
	for _, tt := range tests {
		if got := h.Get(tt.key); got != tt.want {
			t.Errorf("Get(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestHeaderGetNilMap(t *testing.T) {
	h := &Header{}
	if got := h.Get("Content-Type"); got != "" {
		t.Errorf("Get on nil header = %q, want empty", got)
	}
}

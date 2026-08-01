package annette

import "net/http"

type (
	Header struct {
		hdr http.Header
	}
)

func (h *Header) Get(key string) string {
	return h.hdr.Get(key)
}

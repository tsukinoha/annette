package annette

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read error") }

type trackingBody struct {
	io.Reader
	closed int
}

func (b *trackingBody) Close() error {
	b.closed++
	return nil
}

func newTestResponse(code int, body string) *Response {
	return &Response{res: &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader(body)),
	}}
}

func TestResponseBodyCanBeReadRepeatedly(t *testing.T) {
	tb := &trackingBody{Reader: strings.NewReader("hello")}
	r := &Response{res: &http.Response{Body: tb}}

	if got := r.Body(); got != "hello" {
		t.Fatalf("Body() = %q, want %q", got, "hello")
	}
	if got := r.Body(); got != "hello" {
		t.Errorf("second Body() = %q, want %q", got, "hello")
	}
	if got := string(r.Binary()); got != "hello" {
		t.Errorf("Binary() = %q, want %q", got, "hello")
	}
	if tb.closed != 1 {
		t.Errorf("body closed %d times, want 1", tb.closed)
	}
	if err := r.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
}

func TestResponseBinaryReadError(t *testing.T) {
	r := &Response{res: &http.Response{Body: io.NopCloser(errReader{})}}
	got := r.Binary()
	if got == nil || len(got) != 0 {
		t.Errorf("Binary() = %v, want empty non-nil slice", got)
	}
	if r.Err() == nil {
		t.Error("Err() = nil, want error")
	}
}

func TestResponseNilBody(t *testing.T) {
	r := &Response{res: &http.Response{}}
	if got := r.Body(); got != "" {
		t.Errorf("Body() = %q, want empty", got)
	}
	if err := r.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
}

func TestResponseAccessors(t *testing.T) {
	hdr := http.Header{}
	hdr.Set("X-Test", "value")
	r := &Response{res: &http.Response{
		StatusCode:    http.StatusTeapot,
		ContentLength: 42,
		Header:        hdr,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Close:         true,
		Uncompressed:  true,
	}}

	if got := r.StatusCode(); got != http.StatusTeapot {
		t.Errorf("StatusCode() = %d", got)
	}
	if got := r.ContentLength(); got != 42 {
		t.Errorf("ContentLength() = %d", got)
	}
	if got := r.GetHeader().Get("X-Test"); got != "value" {
		t.Errorf("GetHeader().Get() = %q", got)
	}
	if got := r.Proto(); got != "HTTP/1.1" {
		t.Errorf("Proto() = %q", got)
	}
	if got := r.ProtoMajor(); got != 1 {
		t.Errorf("ProtoMajor() = %d", got)
	}
	if got := r.ProtoMinor(); got != 1 {
		t.Errorf("ProtoMinor() = %d", got)
	}
	if !r.Close() {
		t.Error("Close() = false")
	}
	if !r.Uncompressed() {
		t.Error("Uncompressed() = false")
	}
}

func TestResponseStatusClasses(t *testing.T) {
	tests := []struct {
		code                         int
		s100, s200, s300, s400, s500 bool
	}{
		{0, false, false, false, false, false},
		{99, false, false, false, false, false},
		{100, true, false, false, false, false},
		{199, true, false, false, false, false},
		{200, false, true, false, false, false},
		{299, false, true, false, false, false},
		{300, false, false, true, false, false},
		{399, false, false, true, false, false},
		{400, false, false, false, true, false},
		{499, false, false, false, true, false},
		{500, false, false, false, false, true},
		{599, false, false, false, false, true},
		{600, false, false, false, false, false},
	}
	for _, tt := range tests {
		r := newTestResponse(tt.code, "")
		got := [5]bool{r.IsStatus100s(), r.IsStatus200s(), r.IsStatus300s(), r.IsStatus400s(), r.IsStatus500s()}
		want := [5]bool{tt.s100, tt.s200, tt.s300, tt.s400, tt.s500}
		if got != want {
			t.Errorf("code %d: got %v, want %v", tt.code, got, want)
		}
	}
}

func TestResponseIsStatusCode(t *testing.T) {
	checks := map[int]func(*Response) bool{
		100: (*Response).IsStatus100, 101: (*Response).IsStatus101,
		102: (*Response).IsStatus102, 103: (*Response).IsStatus103,
		200: (*Response).IsStatus200, 201: (*Response).IsStatus201,
		202: (*Response).IsStatus202, 203: (*Response).IsStatus203,
		204: (*Response).IsStatus204, 205: (*Response).IsStatus205,
		206: (*Response).IsStatus206, 207: (*Response).IsStatus207,
		208: (*Response).IsStatus208, 226: (*Response).IsStatus226,
		300: (*Response).IsStatus300, 301: (*Response).IsStatus301,
		302: (*Response).IsStatus302, 303: (*Response).IsStatus303,
		304: (*Response).IsStatus304, 305: (*Response).IsStatus305,
		307: (*Response).IsStatus307, 308: (*Response).IsStatus308,
		400: (*Response).IsStatus400, 401: (*Response).IsStatus401,
		402: (*Response).IsStatus402, 403: (*Response).IsStatus403,
		404: (*Response).IsStatus404, 405: (*Response).IsStatus405,
		406: (*Response).IsStatus406, 407: (*Response).IsStatus407,
		408: (*Response).IsStatus408, 409: (*Response).IsStatus409,
		410: (*Response).IsStatus410, 411: (*Response).IsStatus411,
		412: (*Response).IsStatus412, 413: (*Response).IsStatus413,
		414: (*Response).IsStatus414, 415: (*Response).IsStatus415,
		416: (*Response).IsStatus416, 417: (*Response).IsStatus417,
		418: (*Response).IsStatus418, 421: (*Response).IsStatus421,
		422: (*Response).IsStatus422, 423: (*Response).IsStatus423,
		424: (*Response).IsStatus424, 425: (*Response).IsStatus425,
		426: (*Response).IsStatus426, 428: (*Response).IsStatus428,
		429: (*Response).IsStatus429, 431: (*Response).IsStatus431,
		451: (*Response).IsStatus451, 500: (*Response).IsStatus500,
		501: (*Response).IsStatus501, 502: (*Response).IsStatus502,
		503: (*Response).IsStatus503, 504: (*Response).IsStatus504,
		505: (*Response).IsStatus505, 506: (*Response).IsStatus506,
		507: (*Response).IsStatus507, 508: (*Response).IsStatus508,
		510: (*Response).IsStatus510, 511: (*Response).IsStatus511,
	}
	for code, fn := range checks {
		if !fn(newTestResponse(code, "")) {
			t.Errorf("IsStatus%d() = false for status %d", code, code)
		}
		if fn(newTestResponse(code+1000, "")) {
			t.Errorf("IsStatus%d() = true for status %d", code, code+1000)
		}
	}
}

package annette

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type recorded struct {
	method string
	path   string
	header http.Header
	body   []byte
}

func newTestServer(t *testing.T) (*httptest.Server, chan recorded) {
	t.Helper()
	ch := make(chan recorded, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		ch <- recorded{method: r.Method, path: r.URL.Path, header: r.Header.Clone(), body: b}
		w.Header().Set("X-Method", r.Method)
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			w.Write([]byte("ok:" + r.Method))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, ch
}

// newBlockingServer returns a server whose handler blocks until the client
// gives up on the request, so that only the client's timeout can end it.
func newBlockingServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The server notices a client disconnect only after the body is read.
		io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	return srv
}

func parseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	uri, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return uri
}

// readCloser signals on closed when Close is called. Close may be called
// asynchronously by the upload goroutine, so tests wait via waitClosed.
type readCloser struct {
	io.Reader
	once   sync.Once
	closed chan struct{}
}

func newReadCloser(r io.Reader) *readCloser {
	return &readCloser{Reader: r, closed: make(chan struct{})}
}

func (r *readCloser) Close() error {
	r.once.Do(func() { close(r.closed) })
	return nil
}

func (r *readCloser) waitClosed() bool {
	select {
	case <-r.closed:
		return true
	case <-time.After(5 * time.Second):
		return false
	}
}

type failingReader struct {
	data []byte
	err  error
}

func (f *failingReader) Read(p []byte) (int, error) {
	if len(f.data) == 0 {
		return 0, f.err
	}
	n := copy(p, f.data)
	f.data = f.data[n:]
	return n, nil
}

func (f *failingReader) Close() error { return nil }

func TestNew(t *testing.T) {
	c := New()
	if c.ChunkSize != chunkSize {
		t.Errorf("ChunkSize = %d, want %d", c.ChunkSize, chunkSize)
	}
	if c.Header == nil {
		t.Error("Header is nil")
	}
	if got := c.Timeout(); got != defaultTimeout {
		t.Errorf("Timeout() = %v, want %v", got, defaultTimeout)
	}
}

func TestClientMethods(t *testing.T) {
	srv, ch := newTestServer(t)
	c := New()
	uri := parseURL(t, srv.URL)

	tests := []struct {
		name   string
		method string
		call   func() (*Response, error)
		body   string
	}{
		{"Get", http.MethodGet, func() (*Response, error) { return c.Get(uri) }, ""},
		{"Head", http.MethodHead, func() (*Response, error) { return c.Head(uri) }, ""},
		{"Post", http.MethodPost, func() (*Response, error) { return c.Post(uri, strings.NewReader("post-body")) }, "post-body"},
		{"Put", http.MethodPut, func() (*Response, error) { return c.Put(uri, strings.NewReader("put-body")) }, "put-body"},
		{"Patch", http.MethodPatch, func() (*Response, error) { return c.Patch(uri, strings.NewReader("patch-body")) }, "patch-body"},
		{"Delete", http.MethodDelete, func() (*Response, error) { return c.Delete(uri) }, ""},
		{"Options", http.MethodOptions, func() (*Response, error) { return c.Options(uri) }, ""},
		{"Trace", http.MethodTrace, func() (*Response, error) { return c.Trace(uri) }, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.call()
			if err != nil {
				t.Fatal(err)
			}
			got := <-ch
			if got.method != tt.method {
				t.Errorf("method = %q, want %q", got.method, tt.method)
			}
			if string(got.body) != tt.body {
				t.Errorf("request body = %q, want %q", got.body, tt.body)
			}
			if !res.IsStatus200() {
				t.Errorf("status = %d", res.StatusCode())
			}
			if h := res.GetHeader().Get("X-Method"); h != tt.method {
				t.Errorf("X-Method = %q, want %q", h, tt.method)
			}
			wantBody := "ok:" + tt.method
			if tt.method == http.MethodHead {
				wantBody = ""
			}
			if b := res.Body(); b != wantBody {
				t.Errorf("response body = %q, want %q", b, wantBody)
			}
		})
	}
}

func TestClientUsesRequestURI(t *testing.T) {
	srv, ch := newTestServer(t)
	c := New()

	for _, path := range []string{"/a", "/b"} {
		res, err := c.Get(parseURL(t, srv.URL+path))
		if err != nil {
			t.Fatal(err)
		}
		res.Err()
		if got := (<-ch).path; got != path {
			t.Errorf("path = %q, want %q", got, path)
		}
	}
}

func TestClientSendsHeader(t *testing.T) {
	srv, ch := newTestServer(t)
	c := New()
	c.Header.Set("Authorization", "Bearer token")

	if _, err := c.Get(parseURL(t, srv.URL)); err != nil {
		t.Fatal(err)
	}
	got := <-ch
	if v := got.header.Get("Authorization"); v != "Bearer token" {
		t.Errorf("Authorization = %q", v)
	}
}

func TestClientNilHeader(t *testing.T) {
	srv, ch := newTestServer(t)
	c := New()
	c.Header = nil
	uri := parseURL(t, srv.URL)

	if _, err := c.Get(uri); err != nil {
		t.Fatalf("Get: %v", err)
	}
	<-ch
	if _, err := c.UploadByPost(uri, io.NopCloser(strings.NewReader("x"))); err != nil {
		t.Fatalf("UploadByPost: %v", err)
	}
	<-ch
}

func TestClientNilURI(t *testing.T) {
	c := New()

	if _, err := c.Get(nil); err == nil {
		t.Error("Get with nil uri: want error")
	}
	stream := newReadCloser(strings.NewReader("x"))
	if _, err := c.UploadByPost(nil, stream); err == nil {
		t.Error("UploadByPost with nil uri: want error")
	}
	if !stream.waitClosed() {
		t.Error("stream not closed when request creation failed")
	}
}

func TestClientUpload(t *testing.T) {
	// Data that is not a multiple of the chunk size, to catch writes of the
	// whole buffer instead of only the bytes actually read.
	data := bytes.Repeat([]byte("0123456789"), 1000) // 10000 bytes

	srv, ch := newTestServer(t)
	c := New()
	uri := parseURL(t, srv.URL)

	tests := []struct {
		name      string
		method    string
		chunkSize int
		call      func(*url.URL, io.ReadCloser) (*Response, error)
	}{
		{"UploadByPost", http.MethodPost, 3, c.UploadByPost},
		{"UploadByPut", http.MethodPut, 4096, c.UploadByPut},
		{"UploadByPatch", http.MethodPatch, 0, c.UploadByPatch}, // falls back to default
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.ChunkSize = tt.chunkSize
			stream := newReadCloser(bytes.NewReader(data))
			res, err := tt.call(uri, stream)
			if err != nil {
				t.Fatal(err)
			}
			got := <-ch
			if got.method != tt.method {
				t.Errorf("method = %q, want %q", got.method, tt.method)
			}
			if !bytes.Equal(got.body, data) {
				t.Errorf("uploaded %d bytes, want %d (content mismatch)", len(got.body), len(data))
			}
			if ct := got.header.Get("Content-Type"); ct != "application/octet-stream" {
				t.Errorf("Content-Type = %q", ct)
			}
			if !res.IsStatus200() {
				t.Errorf("status = %d", res.StatusCode())
			}
			if !stream.waitClosed() {
				t.Error("stream not closed")
			}
			if c.ChunkSize != tt.chunkSize {
				t.Errorf("ChunkSize mutated to %d", c.ChunkSize)
			}
		})
	}
}

func TestClientUploadDoesNotMutateHeader(t *testing.T) {
	srv, ch := newTestServer(t)
	c := New()
	uri := parseURL(t, srv.URL)

	if _, err := c.UploadByPost(uri, io.NopCloser(strings.NewReader("data"))); err != nil {
		t.Fatal(err)
	}
	<-ch
	if ct := c.Header.Get("Content-Type"); ct != "" {
		t.Errorf("client Header Content-Type = %q, want empty", ct)
	}

	if _, err := c.Get(uri); err != nil {
		t.Fatal(err)
	}
	got := <-ch
	if ct := got.header.Get("Content-Type"); ct != "" {
		t.Errorf("Get sent Content-Type = %q, want empty", ct)
	}
}

func TestClientUploadStreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body)
	}))
	t.Cleanup(srv.Close)
	c := New()

	stream := &failingReader{data: []byte("partial"), err: errors.New("disk failure")}
	if _, err := c.UploadByPost(parseURL(t, srv.URL), stream); err == nil {
		t.Error("want error when stream read fails, got nil")
	}
}

func TestClientUploadNilStream(t *testing.T) {
	c := New()
	if _, err := c.UploadByPost(parseURL(t, "http://example.com"), nil); err == nil {
		t.Error("want error for nil stream")
	}
}

// isTimeout reports whether err comes from the client's timeout.
func isTimeout(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}

func TestClientSetTimeout(t *testing.T) {
	tests := []struct {
		name   string
		second int
		want   time.Duration
	}{
		{"positive", 3, 3 * time.Second},
		{"zero means no timeout", 0, 0},
		{"negative restores default", -1, defaultTimeout},
		{"too large does not overflow", math.MaxInt, time.Duration(math.MaxInt64)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New()
			c.SetTimeout(10) // start from a non-default value
			c.SetTimeout(tt.second)
			if got := c.Timeout(); got != tt.want {
				t.Errorf("SetTimeout(%d): Timeout() = %v, want %v", tt.second, got, tt.want)
			}
		})
	}
}

func TestClientSetTimeoutApplies(t *testing.T) {
	// End-to-end check through SetTimeout. It takes one second, the shortest
	// timeout SetTimeout can set; the other tests set c.timeout directly.
	srv := newBlockingServer(t)
	c := New()
	c.SetTimeout(1)

	start := time.Now()
	if _, err := c.Get(parseURL(t, srv.URL)); !isTimeout(err) {
		t.Errorf("Get err = %v, want timeout", err)
	}
	if d := time.Since(start); d < time.Second || d > 5*time.Second {
		t.Errorf("Get took %v, want about 1s", d)
	}
}

func TestClientTimeout(t *testing.T) {
	srv := newBlockingServer(t)
	c := New()
	c.timeout = 50 * time.Millisecond
	uri := parseURL(t, srv.URL)

	if _, err := c.Get(uri); !isTimeout(err) {
		t.Errorf("Get err = %v, want timeout", err)
	}

	pr, pw := io.Pipe()
	defer pw.Close()
	stream := newReadCloser(pr)
	if _, err := c.UploadByPut(uri, stream); !isTimeout(err) {
		t.Errorf("UploadByPut err = %v, want timeout", err)
	}
	pw.Close()
	if !stream.waitClosed() {
		t.Error("stream not closed after timeout")
	}
}

func TestClientZeroTimeout(t *testing.T) {
	// Zero means no timeout, so requests must not fail immediately.
	srv, ch := newTestServer(t)
	c := New()
	c.SetTimeout(0)
	uri := parseURL(t, srv.URL)

	res, err := c.Get(uri)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	<-ch
	if b := res.Body(); b != "ok:GET" {
		t.Errorf("body = %q", b)
	}

	res, err = c.UploadByPost(uri, io.NopCloser(strings.NewReader("data")))
	if err != nil {
		t.Fatalf("UploadByPost: %v", err)
	}
	<-ch
	if b := res.Body(); b != "ok:POST" {
		t.Errorf("body = %q", b)
	}
}

func TestClientTimeoutCoversResponseBody(t *testing.T) {
	// The server sends the headers, then never sends the body.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	c := New()
	c.timeout = 100 * time.Millisecond

	res, err := c.Get(parseURL(t, srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	if err := res.Err(); !isTimeout(err) {
		t.Errorf("Err() = %v, want timeout", err)
	}
}

func TestClientBodyReadableAfterReturn(t *testing.T) {
	// The body is read after the request method has returned. It must not be
	// cut off by anything that ends when the method returns.
	body := strings.Repeat("a", 1<<20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c := New()
	uri := parseURL(t, srv.URL)

	for i := 0; i < 20; i++ {
		res, err := c.Get(uri)
		if err != nil {
			t.Fatal(err)
		}
		if err := res.Err(); err != nil {
			t.Fatalf("Get #%d: Err() = %v", i, err)
		}
		if n := len(res.Binary()); n != len(body) {
			t.Fatalf("Get #%d: len = %d, want %d", i, n, len(body))
		}

		res, err = c.UploadByPost(uri, io.NopCloser(strings.NewReader("x")))
		if err != nil {
			t.Fatal(err)
		}
		if n := len(res.Binary()); n != len(body) {
			t.Fatalf("UploadByPost #%d: len = %d, want %d (err %v)", i, n, len(body), res.Err())
		}
	}
}

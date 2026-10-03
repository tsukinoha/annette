package annette

import (
	"bytes"
	"context"
	"errors"
	"io"
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
	header http.Header
	body   []byte
}

func newTestServer(t *testing.T) (*httptest.Server, chan recorded) {
	t.Helper()
	ch := make(chan recorded, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		ch <- recorded{method: r.Method, header: r.Header.Clone(), body: b}
		w.Header().Set("X-Method", r.Method)
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			w.Write([]byte("ok:" + r.Method))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, ch
}

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	uri, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return New(uri)
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
	uri, _ := url.Parse("http://example.com")
	c := New(uri)
	if c.uri != uri {
		t.Error("uri not set")
	}
	if c.ChunkSize != chunkSize {
		t.Errorf("ChunkSize = %d, want %d", c.ChunkSize, chunkSize)
	}
	if c.Header == nil {
		t.Error("Header is nil")
	}
	if c.Context == nil {
		t.Error("Context is nil")
	}
}

func TestClientMethods(t *testing.T) {
	srv, ch := newTestServer(t)
	c := newTestClient(t, srv)

	tests := []struct {
		name   string
		method string
		call   func() (*Response, error)
		body   string
	}{
		{"Get", http.MethodGet, c.Get, ""},
		{"Head", http.MethodHead, c.Head, ""},
		{"Post", http.MethodPost, func() (*Response, error) { return c.Post(strings.NewReader("post-body")) }, "post-body"},
		{"Put", http.MethodPut, func() (*Response, error) { return c.Put(strings.NewReader("put-body")) }, "put-body"},
		{"Patch", http.MethodPatch, func() (*Response, error) { return c.Patch(strings.NewReader("patch-body")) }, "patch-body"},
		{"Delete", http.MethodDelete, c.Delete, ""},
		{"Options", http.MethodOptions, c.Options, ""},
		{"Trace", http.MethodTrace, c.Trace, ""},
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

func TestClientSendsHeader(t *testing.T) {
	srv, ch := newTestServer(t)
	c := newTestClient(t, srv)
	c.Header.Set("Authorization", "Bearer token")

	if _, err := c.Get(); err != nil {
		t.Fatal(err)
	}
	got := <-ch
	if v := got.header.Get("Authorization"); v != "Bearer token" {
		t.Errorf("Authorization = %q", v)
	}
}

func TestClientNilHeaderAndContext(t *testing.T) {
	srv, ch := newTestServer(t)
	c := newTestClient(t, srv)
	c.Header = nil
	c.Context = nil //nolint:staticcheck // intentionally testing nil context

	if _, err := c.Get(); err != nil {
		t.Fatalf("Get: %v", err)
	}
	<-ch
	if _, err := c.UploadByPost(io.NopCloser(strings.NewReader("x"))); err != nil {
		t.Fatalf("UploadByPost: %v", err)
	}
	<-ch
}

func TestClientNilURI(t *testing.T) {
	c := New(nil)
	if _, err := c.Get(); err == nil {
		t.Error("Get with nil uri: want error")
	}
	stream := newReadCloser(strings.NewReader("x"))
	if _, err := c.UploadByPost(stream); err == nil {
		t.Error("UploadByPost with nil uri: want error")
	}
	if !stream.waitClosed() {
		t.Error("stream not closed when request creation failed")
	}
}

func TestClientContextCanceled(t *testing.T) {
	srv, _ := newTestServer(t)
	c := newTestClient(t, srv)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Context = ctx

	if _, err := c.Get(); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestClientUpload(t *testing.T) {
	// Data that is not a multiple of the chunk size, to catch writes of the
	// whole buffer instead of only the bytes actually read.
	data := bytes.Repeat([]byte("0123456789"), 1000) // 10000 bytes

	srv, ch := newTestServer(t)
	c := newTestClient(t, srv)

	tests := []struct {
		name      string
		method    string
		chunkSize int
		call      func(io.ReadCloser) (*Response, error)
	}{
		{"UploadByPost", http.MethodPost, 3, c.UploadByPost},
		{"UploadByPut", http.MethodPut, 4096, c.UploadByPut},
		{"UploadByPatch", http.MethodPatch, 0, c.UploadByPatch}, // falls back to default
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.ChunkSize = tt.chunkSize
			stream := newReadCloser(bytes.NewReader(data))
			res, err := tt.call(stream)
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
	c := newTestClient(t, srv)

	if _, err := c.UploadByPost(io.NopCloser(strings.NewReader("data"))); err != nil {
		t.Fatal(err)
	}
	<-ch
	if ct := c.Header.Get("Content-Type"); ct != "" {
		t.Errorf("client Header Content-Type = %q, want empty", ct)
	}

	if _, err := c.Get(); err != nil {
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
	c := newTestClient(t, srv)

	stream := &failingReader{data: []byte("partial"), err: errors.New("disk failure")}
	if _, err := c.UploadByPost(stream); err == nil {
		t.Error("want error when stream read fails, got nil")
	}
}

func TestClientUploadNilStream(t *testing.T) {
	c := New(&url.URL{Scheme: "http", Host: "example.com"})
	if _, err := c.UploadByPost(nil); err == nil {
		t.Error("want error for nil stream")
	}
}

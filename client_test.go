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
// gives up on the request, so that only the client's context can end it.
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
}

func TestClientMethods(t *testing.T) {
	srv, ch := newTestServer(t)
	c := New()
	ctx := context.Background()
	uri := parseURL(t, srv.URL)

	tests := []struct {
		name   string
		method string
		call   func() (*Response, error)
		body   string
	}{
		{"Get", http.MethodGet, func() (*Response, error) { return c.Get(ctx, uri) }, ""},
		{"Head", http.MethodHead, func() (*Response, error) { return c.Head(ctx, uri) }, ""},
		{"Post", http.MethodPost, func() (*Response, error) { return c.Post(ctx, uri, strings.NewReader("post-body")) }, "post-body"},
		{"Put", http.MethodPut, func() (*Response, error) { return c.Put(ctx, uri, strings.NewReader("put-body")) }, "put-body"},
		{"Patch", http.MethodPatch, func() (*Response, error) { return c.Patch(ctx, uri, strings.NewReader("patch-body")) }, "patch-body"},
		{"Delete", http.MethodDelete, func() (*Response, error) { return c.Delete(ctx, uri) }, ""},
		{"Options", http.MethodOptions, func() (*Response, error) { return c.Options(ctx, uri) }, ""},
		{"Trace", http.MethodTrace, func() (*Response, error) { return c.Trace(ctx, uri) }, ""},
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
		res, err := c.Get(context.Background(), parseURL(t, srv.URL+path))
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

	if _, err := c.Get(context.Background(), parseURL(t, srv.URL)); err != nil {
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
	ctx := context.Background()
	uri := parseURL(t, srv.URL)

	if _, err := c.Get(ctx, uri); err != nil {
		t.Fatalf("Get: %v", err)
	}
	<-ch
	if _, err := c.UploadByPost(ctx, uri, io.NopCloser(strings.NewReader("x"))); err != nil {
		t.Fatalf("UploadByPost: %v", err)
	}
	<-ch
}

func TestClientNilContext(t *testing.T) {
	c := New()
	uri := parseURL(t, "http://example.com")

	if _, err := c.Get(nil, uri); err == nil { //nolint:staticcheck // intentionally testing nil context
		t.Error("Get with nil ctx: want error")
	}
	stream := newReadCloser(strings.NewReader("x"))
	if _, err := c.UploadByPost(nil, uri, stream); err == nil { //nolint:staticcheck // intentionally testing nil context
		t.Error("UploadByPost with nil ctx: want error")
	}
	if !stream.waitClosed() {
		t.Error("stream not closed when request creation failed")
	}
}

func TestClientNilURI(t *testing.T) {
	c := New()
	ctx := context.Background()

	if _, err := c.Get(ctx, nil); err == nil {
		t.Error("Get with nil uri: want error")
	}
	stream := newReadCloser(strings.NewReader("x"))
	if _, err := c.UploadByPost(ctx, nil, stream); err == nil {
		t.Error("UploadByPost with nil uri: want error")
	}
	if !stream.waitClosed() {
		t.Error("stream not closed when request creation failed")
	}
}

func TestClientContextCanceled(t *testing.T) {
	srv, _ := newTestServer(t)
	c := New()
	uri := parseURL(t, srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.Get(ctx, uri); !errors.Is(err, context.Canceled) {
		t.Errorf("Get err = %v, want context.Canceled", err)
	}
	stream := newReadCloser(strings.NewReader("x"))
	if _, err := c.UploadByPost(ctx, uri, stream); !errors.Is(err, context.Canceled) {
		t.Errorf("UploadByPost err = %v, want context.Canceled", err)
	}
	if !stream.waitClosed() {
		t.Error("stream not closed")
	}
}

func TestClientContextDeadline(t *testing.T) {
	srv := newBlockingServer(t)
	c := New()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if _, err := c.Get(ctx, parseURL(t, srv.URL)); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestClientContextCanceledDuringUpload(t *testing.T) {
	srv := newBlockingServer(t)
	c := New()
	ctx, cancel := context.WithCancel(context.Background())

	// The stream never produces data until pw is closed, so the upload can
	// only end through ctx.
	pr, pw := io.Pipe()
	stream := newReadCloser(pr)
	time.AfterFunc(50*time.Millisecond, cancel)

	if _, err := c.UploadByPut(ctx, parseURL(t, srv.URL), stream); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	// Let the blocked Read return so the upload goroutine can finish.
	pw.Close()
	if !stream.waitClosed() {
		t.Error("stream not closed after cancel")
	}
}

func TestClientContextPerRequest(t *testing.T) {
	// One client is shared by requests with different contexts. Canceling
	// one request's context must not affect the other.
	srv, ch := newTestServer(t)
	c := New()
	uri := parseURL(t, srv.URL)

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Get(canceled, uri); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled Get err = %v, want context.Canceled", err)
	}

	res, err := c.Get(context.Background(), uri)
	if err != nil {
		t.Fatalf("Get after another request was canceled: %v", err)
	}
	<-ch
	if b := res.Body(); b != "ok:GET" {
		t.Errorf("body = %q", b)
	}
}

func TestClientContextCoversResponseBody(t *testing.T) {
	// The server sends the headers, then waits before sending the body.
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		select {
		case <-release:
			w.Write([]byte("late body"))
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	c := New()

	ctx, cancel := context.WithCancel(context.Background())
	res, err := c.Get(ctx, parseURL(t, srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	// Canceling before the body is read aborts the body read.
	cancel()
	if err := res.Err(); !errors.Is(err, context.Canceled) {
		t.Errorf("Err() = %v, want context.Canceled", err)
	}
}

func TestClientUpload(t *testing.T) {
	// Data that is not a multiple of the chunk size, to catch writes of the
	// whole buffer instead of only the bytes actually read.
	data := bytes.Repeat([]byte("0123456789"), 1000) // 10000 bytes

	srv, ch := newTestServer(t)
	c := New()
	ctx := context.Background()
	uri := parseURL(t, srv.URL)

	tests := []struct {
		name      string
		method    string
		chunkSize int
		call      func(context.Context, *url.URL, io.ReadCloser) (*Response, error)
	}{
		{"UploadByPost", http.MethodPost, 3, c.UploadByPost},
		{"UploadByPut", http.MethodPut, 4096, c.UploadByPut},
		{"UploadByPatch", http.MethodPatch, 0, c.UploadByPatch}, // falls back to default
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.ChunkSize = tt.chunkSize
			stream := newReadCloser(bytes.NewReader(data))
			res, err := tt.call(ctx, uri, stream)
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
	ctx := context.Background()
	uri := parseURL(t, srv.URL)

	if _, err := c.UploadByPost(ctx, uri, io.NopCloser(strings.NewReader("data"))); err != nil {
		t.Fatal(err)
	}
	<-ch
	if ct := c.Header.Get("Content-Type"); ct != "" {
		t.Errorf("client Header Content-Type = %q, want empty", ct)
	}

	if _, err := c.Get(ctx, uri); err != nil {
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
	if _, err := c.UploadByPost(context.Background(), parseURL(t, srv.URL), stream); err == nil {
		t.Error("want error when stream read fails, got nil")
	}
}

func TestClientUploadNilStream(t *testing.T) {
	c := New()
	if _, err := c.UploadByPost(context.Background(), parseURL(t, "http://example.com"), nil); err == nil {
		t.Error("want error for nil stream")
	}
}

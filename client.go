package annette

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"time"
)

const (
	chunkSize int = 4194304 // 4MB

	// defaultTimeout is the default value of the client's timeout.
	defaultTimeout = 30 * time.Second

	// maxTimeoutSeconds is the largest number of seconds a time.Duration holds.
	maxTimeoutSeconds = int64(math.MaxInt64 / time.Second)
)

type (
	// Client is a HTTP client structure in Annette package.
	// It holds no per-request state, so one Client can be reused for many
	// requests. Each request method takes the URL to use.
	//
	// Each request is limited by the client's timeout (30 seconds by default,
	// see SetTimeout). The timeout covers the whole request: connecting,
	// sending the request body and reading the response body with
	// Response.Body, Binary or Err. When it expires the error wraps
	// context.DeadlineExceeded.
	Client struct {
		ChunkSize int
		Header    http.Header
		timeout   time.Duration
	}
)

func New() *Client {
	return &Client{
		ChunkSize: chunkSize,
		Header:    http.Header{},
		timeout:   defaultTimeout,
	}
}

func (c *Client) newRequest(ctx context.Context, method string, uri *url.URL, body io.Reader) (*http.Request, error) {
	if uri == nil {
		return nil, errors.New("annette: uri is nil")
	}
	req, err := http.NewRequestWithContext(ctx, method, uri.String(), body)
	if err != nil {
		return nil, err
	}
	// Clone so that per-request changes never leak into the client's header.
	req.Header = c.Header.Clone()
	if req.Header == nil {
		req.Header = http.Header{}
	}
	return req, nil
}

// withTimeout returns a context that ends when the client's timeout expires.
// The returned cancel must be called once the response body is no longer
// needed, not when the request method returns, because the body is read
// afterwards. http.Client.Timeout is not used because it does not stop the
// upload goroutine when a stream blocks.
func (c *Client) withTimeout() (context.Context, context.CancelFunc) {
	if c.timeout <= 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), c.timeout)
}

// cancelBody calls cancel when the response body is closed, so that the
// timeout context lives until the body has been read.
type cancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

// do sends req and takes ownership of cancel: it is called on error, or when
// the response body is closed.
func (c *Client) do(req *http.Request, cancel context.CancelFunc) (*Response, error) {
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	if res.Body == nil {
		cancel()
	} else {
		res.Body = &cancelBody{ReadCloser: res.Body, cancel: cancel}
	}
	return &Response{res: res}, nil
}

func (c *Client) send(method string, uri *url.URL, body io.Reader) (*Response, error) {
	ctx, cancel := c.withTimeout()
	req, err := c.newRequest(ctx, method, uri, body)
	if err != nil {
		cancel()
		return nil, err
	}
	return c.do(req, cancel)
}

func (c *Client) upload(method string, uri *url.URL, stream io.ReadCloser) (*Response, error) {
	if stream == nil {
		return nil, errors.New("annette: stream is nil")
	}
	ctx, cancel := c.withTimeout()
	r, w := io.Pipe()
	req, err := c.newRequest(ctx, method, uri, r)
	if err != nil {
		cancel()
		stream.Close()
		return nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	size := c.ChunkSize
	if size < 1 {
		size = chunkSize
	}
	// The transport waits for the body to be written before it returns, even
	// after ctx is done, so break the pipe on cancel to unblock it.
	stop := context.AfterFunc(ctx, func() {
		r.CloseWithError(context.Cause(ctx))
	})
	go func() {
		defer stop()
		defer stream.Close()
		b := make([]byte, size)
		_, err := io.CopyBuffer(w, struct{ io.Reader }{stream}, b)
		// CloseWithError(nil) behaves like Close (reader receives io.EOF).
		w.CloseWithError(err)
	}()

	res, err := c.do(req, cancel)
	if err != nil {
		// Unblock the writer goroutine if the transport did not consume the body.
		r.CloseWithError(err)
		return nil, err
	}
	return res, nil
}

func (c *Client) Get(uri *url.URL) (*Response, error) {
	return c.send(http.MethodGet, uri, nil)
}

func (c *Client) Head(uri *url.URL) (*Response, error) {
	return c.send(http.MethodHead, uri, nil)
}

func (c *Client) Post(uri *url.URL, body io.Reader) (*Response, error) {
	return c.send(http.MethodPost, uri, body)
}

func (c *Client) Put(uri *url.URL, body io.Reader) (*Response, error) {
	return c.send(http.MethodPut, uri, body)
}

func (c *Client) Patch(uri *url.URL, body io.Reader) (*Response, error) {
	return c.send(http.MethodPatch, uri, body)
}

func (c *Client) Delete(uri *url.URL) (*Response, error) {
	return c.send(http.MethodDelete, uri, nil)
}

func (c *Client) Options(uri *url.URL) (*Response, error) {
	return c.send(http.MethodOptions, uri, nil)
}

func (c *Client) Trace(uri *url.URL) (*Response, error) {
	return c.send(http.MethodTrace, uri, nil)
}

func (c *Client) UploadByPost(uri *url.URL, stream io.ReadCloser) (*Response, error) {
	return c.upload(http.MethodPost, uri, stream)
}

func (c *Client) UploadByPut(uri *url.URL, stream io.ReadCloser) (*Response, error) {
	return c.upload(http.MethodPut, uri, stream)
}

func (c *Client) UploadByPatch(uri *url.URL, stream io.ReadCloser) (*Response, error) {
	return c.upload(http.MethodPatch, uri, stream)
}

// SetTimeout sets the time limit, in seconds, for each request made by the
// client, including reading the response body. Zero means no timeout, and a
// negative value restores the default of 30 seconds.
//
// The timeout applies to uploads as well, so set it long enough (or to zero)
// for large uploads. Call SetTimeout before making requests; it must not be
// called while requests are in progress.
func (c *Client) SetTimeout(second int) {
	switch {
	case second < 0:
		c.timeout = defaultTimeout
	case int64(second) > maxTimeoutSeconds:
		// time.Duration(second) * time.Second would overflow.
		c.timeout = time.Duration(math.MaxInt64)
	default:
		c.timeout = time.Duration(second) * time.Second
	}
}

// Timeout returns the time limit set by SetTimeout.
func (c *Client) Timeout() time.Duration {
	return c.timeout
}

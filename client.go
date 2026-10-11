package annette

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
)

const (
	chunkSize int = 4194304 // 4MB
)

type (
	// Client is a HTTP client structure in Annette package.
	// It holds no per-request state, so one Client can be reused for many
	// requests. Each request method takes the context and URL to use.
	// The context also covers reading the response body, so do not cancel it
	// until the body has been read with Response.Body, Binary or Err.
	Client struct {
		ChunkSize int
		Header    http.Header
	}
)

func New() *Client {
	return &Client{
		ChunkSize: chunkSize,
		Header:    http.Header{},
	}
}

func (c *Client) newRequest(ctx context.Context, method string, uri *url.URL, body io.Reader) (*http.Request, error) {
	if ctx == nil {
		return nil, errors.New("annette: ctx is nil")
	}
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

func (c *Client) do(req *http.Request) (*Response, error) {
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	return &Response{res: res}, nil
}

func (c *Client) send(ctx context.Context, method string, uri *url.URL, body io.Reader) (*Response, error) {
	req, err := c.newRequest(ctx, method, uri, body)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *Client) upload(ctx context.Context, method string, uri *url.URL, stream io.ReadCloser) (*Response, error) {
	if stream == nil {
		return nil, errors.New("annette: stream is nil")
	}
	r, w := io.Pipe()
	req, err := c.newRequest(ctx, method, uri, r)
	if err != nil {
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

	res, err := c.do(req)
	if err != nil {
		// Unblock the writer goroutine if the transport did not consume the body.
		r.CloseWithError(err)
		return nil, err
	}
	return res, nil
}

func (c *Client) Get(ctx context.Context, uri *url.URL) (*Response, error) {
	return c.send(ctx, http.MethodGet, uri, nil)
}

func (c *Client) Head(ctx context.Context, uri *url.URL) (*Response, error) {
	return c.send(ctx, http.MethodHead, uri, nil)
}

func (c *Client) Post(ctx context.Context, uri *url.URL, body io.Reader) (*Response, error) {
	return c.send(ctx, http.MethodPost, uri, body)
}

func (c *Client) Put(ctx context.Context, uri *url.URL, body io.Reader) (*Response, error) {
	return c.send(ctx, http.MethodPut, uri, body)
}

func (c *Client) Patch(ctx context.Context, uri *url.URL, body io.Reader) (*Response, error) {
	return c.send(ctx, http.MethodPatch, uri, body)
}

func (c *Client) Delete(ctx context.Context, uri *url.URL) (*Response, error) {
	return c.send(ctx, http.MethodDelete, uri, nil)
}

func (c *Client) Options(ctx context.Context, uri *url.URL) (*Response, error) {
	return c.send(ctx, http.MethodOptions, uri, nil)
}

func (c *Client) Trace(ctx context.Context, uri *url.URL) (*Response, error) {
	return c.send(ctx, http.MethodTrace, uri, nil)
}

func (c *Client) UploadByPost(ctx context.Context, uri *url.URL, stream io.ReadCloser) (*Response, error) {
	return c.upload(ctx, http.MethodPost, uri, stream)
}

func (c *Client) UploadByPut(ctx context.Context, uri *url.URL, stream io.ReadCloser) (*Response, error) {
	return c.upload(ctx, http.MethodPut, uri, stream)
}

func (c *Client) UploadByPatch(ctx context.Context, uri *url.URL, stream io.ReadCloser) (*Response, error) {
	return c.upload(ctx, http.MethodPatch, uri, stream)
}

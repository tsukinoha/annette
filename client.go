package annette

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
)

const (
	chunkSize int = 1048576 // 1MB
)

type (
	// Client is a HTTP client structure in Annette package
	Client struct {
		uri       *url.URL
		ChunkSize int
		Header    http.Header
		Context   context.Context
	}
)

func New(uri *url.URL) *Client {
	return &Client{
		uri:       uri,
		ChunkSize: chunkSize,
		Header:    http.Header{},
		Context:   context.Background(),
	}
}

func (c *Client) newRequest(method string, body io.Reader) (*http.Request, error) {
	if c.uri == nil {
		return nil, errors.New("annette: uri is nil")
	}
	ctx := c.Context
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, method, c.uri.String(), body)
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

func (c *Client) send(method string, body io.Reader) (*Response, error) {
	req, err := c.newRequest(method, body)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *Client) upload(method string, stream io.ReadCloser) (*Response, error) {
	if stream == nil {
		return nil, errors.New("annette: stream is nil")
	}
	r, w := io.Pipe()
	req, err := c.newRequest(method, r)
	if err != nil {
		stream.Close()
		return nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	size := c.ChunkSize
	if size < 1 {
		size = chunkSize
	}
	go func() {
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

func (c *Client) Get() (*Response, error) {
	return c.send(http.MethodGet, nil)
}

func (c *Client) Head() (*Response, error) {
	return c.send(http.MethodHead, nil)
}

func (c *Client) Post(body io.Reader) (*Response, error) {
	return c.send(http.MethodPost, body)
}

func (c *Client) Put(body io.Reader) (*Response, error) {
	return c.send(http.MethodPut, body)
}

func (c *Client) Patch(body io.Reader) (*Response, error) {
	return c.send(http.MethodPatch, body)
}

func (c *Client) Delete() (*Response, error) {
	return c.send(http.MethodDelete, nil)
}

func (c *Client) Options() (*Response, error) {
	return c.send(http.MethodOptions, nil)
}

func (c *Client) Trace() (*Response, error) {
	return c.send(http.MethodTrace, nil)
}

func (c *Client) UploadByPost(stream io.ReadCloser) (*Response, error) {
	return c.upload(http.MethodPost, stream)
}

func (c *Client) UploadByPut(stream io.ReadCloser) (*Response, error) {
	return c.upload(http.MethodPut, stream)
}

func (c *Client) UploadByPatch(stream io.ReadCloser) (*Response, error) {
	return c.upload(http.MethodPatch, stream)
}

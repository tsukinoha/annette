package annette

import (
	"context"
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

func (c *Client) send(method string, body io.Reader) (*Response, error) {
	req, err := http.NewRequestWithContext(c.Context, method, c.uri.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header = c.Header
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	return &Response{res: res}, nil
}

func (c *Client) upload(method string, stream io.ReadCloser) (*Response, error) {
	r, w := io.Pipe()
	req, err := http.NewRequestWithContext(c.Context, method, c.uri.String(), r)
	if err != nil {
		return nil, err
	}

	go func() {
		defer w.Close()
		defer stream.Close()

		if c.ChunkSize < 1 {
			c.ChunkSize = chunkSize
		}
		b := make([]byte, c.ChunkSize)
		for {
			_, err := stream.Read(b)
			if err == io.EOF {
				return
			}
			if err != nil {
				return
			}
			w.Write(b)
		}
	}()

	c.Header.Set("Content-type", "application/octet-stream")
	req.Header = c.Header
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	return &Response{res: res}, nil
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

# Annette -HTTP Client-

Annette is a simple HTTP client for Go.

It wraps `net/http` with a small API: build a `Client`, call a method such as
`Get(ctx, uri)` or `Post(ctx, uri, body)`, and read the result through helper methods
like `Body()` and `IsStatus200()`. It can also stream large uploads in chunks
without loading the whole payload into memory.

## Features

- One method per HTTP verb: `Get`, `Head`, `Post`, `Put`, `Patch`, `Delete`,
  `Options` and `Trace`
- Streaming uploads (`UploadByPost` / `UploadByPut` / `UploadByPatch`) with a
  configurable chunk size
- Default headers for each client and a `context.Context` for each request
- Response helpers for the body, headers, protocol and status code checks
  (by class, such as `IsStatus400s()`, or by exact code, such as `IsStatus404()`)
- No dependencies outside the Go standard library

## Requirements

- Go 1.26 or later

## Installation

```sh
go get github.com/tsukinoha/annette
```

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/tsukinoha/annette"
)

func main() {
	// The parent context for the whole program. Calling cancel stops every
	// request that was started from it.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	uri, err := url.Parse("https://example.com")
	if err != nil {
		log.Fatal(err)
	}

	client := annette.New()
	if err := fetch(ctx, client, uri); err != nil {
		log.Fatal(err)
	}
}

func fetch(ctx context.Context, client *annette.Client, uri *url.URL) error {
	// A timeout for this request only, derived from the parent context.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	res, err := client.Get(ctx, uri)
	if err != nil {
		return err
	}

	// Read the body before cancel runs (see "Timeouts and cancellation").
	body := res.Body()
	if err := res.Err(); err != nil {
		return err
	}

	fmt.Println(res.StatusCode())
	fmt.Println(res.GetHeader().Get("Content-Type"))
	fmt.Println(body)
	return nil
}
```

## Usage

### Creating a client

`New` creates a client. The URL and the context are not stored in the client;
you pass them to each request, so one client can be shared by many requests
(and goroutines) to different URLs. The client has these public fields:

| Field       | Type              | Default                | Description                              |
|-------------|-------------------|------------------------|------------------------------------------|
| `Header`    | `http.Header`     | empty                  | Headers sent with every request          |
| `ChunkSize` | `int`             | `4194304` (4 MB)       | Buffer size for streaming uploads        |

```go
client := annette.New()

client.Header.Set("Authorization", "Bearer <token>")
client.Header.Set("Accept", "application/json")
```

Each request uses a copy of `client.Header`, so a request never changes the
client's headers.

### Sending requests

In the examples below, `ctx` is a per-request context created with
`context.WithTimeout` from the parent context (see
[Timeouts and cancellation](#timeouts-and-cancellation)).

```go
ctx, cancel := context.WithTimeout(parent, 10*time.Second)
defer cancel()

uri, _ := url.Parse("https://api.example.com/items")

res, err := client.Get(ctx, uri)
res, err := client.Head(ctx, uri)
res, err := client.Delete(ctx, uri)
res, err := client.Options(ctx, uri)
res, err := client.Trace(ctx, uri)

// Methods with a body take an io.Reader.
res, err := client.Post(ctx, uri, strings.NewReader(`{"name":"annette"}`))
res, err := client.Put(ctx, uri, bytes.NewReader(data))
res, err := client.Patch(ctx, uri, strings.NewReader(`{"name":"new"}`))
```

The `Content-Type` header is not set for you. Set it in `client.Header` when
the server needs it:

```go
client.Header.Set("Content-Type", "application/json")
res, err := client.Post(ctx, uri, strings.NewReader(`{"name":"annette"}`))
```

### Timeouts and cancellation

Every request method takes a `context.Context`. The context must not be
`nil`. A good pattern uses two levels:

1. In `main` (or wherever your program's work starts), create one parent
   context with `context.WithCancel`. Calling its `cancel` stops all requests
   that are still running, for example on shutdown.
2. For each request, create a child context with `context.WithTimeout` from the
   parent. It limits how long that one request may take, and it is still
   canceled when the parent is canceled.

```go
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := annette.New()
	for _, raw := range []string{"https://example.com/a", "https://example.com/b"} {
		uri, err := url.Parse(raw)
		if err != nil {
			log.Fatal(err)
		}
		body, err := get(ctx, client, uri)
		if err != nil {
			log.Println(err) // wraps context.DeadlineExceeded after 10 seconds
			continue
		}
		fmt.Println(body)
	}
}

func get(ctx context.Context, client *annette.Client, uri *url.URL) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	res, err := client.Get(ctx, uri)
	if err != nil {
		return "", err
	}
	body := res.Body()
	return body, res.Err()
}
```

Create the per-request context in its own function, as `get` does above, so
that `defer cancel()` runs when each request ends. A `defer` inside the `for`
loop in `main` would not run until `main` returns.

The context also covers reading the response body. If the context is
canceled before you call `Body()`, `Binary()` or `Err()`, the read fails and
`Err()` returns the context's error. Read the body before `cancel` runs, as
`get` does above.

### Uploading large files

The upload methods stream data from an `io.ReadCloser` to the server in chunks
of `ChunkSize` bytes, so the whole file is never held in memory. The request is
sent with `Content-Type: application/octet-stream`.

Large uploads can take a long time, so give them a longer timeout than other
requests:

```go
func upload(ctx context.Context, client *annette.Client, uri *url.URL, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	client.ChunkSize = 1024 * 1024               // 1 MB (optional)
	res, err := client.UploadByPut(ctx, uri, f) // or UploadByPost / UploadByPatch
	if err != nil {
		return err
	}
	if err := res.Err(); err != nil {
		return err
	}
	fmt.Println(res.StatusCode())
	return nil
}
```

- The client closes the stream for you, including when the request fails.
- If reading the stream fails partway, the request fails as well, so a
  partial upload is not reported as a success.
- If `ChunkSize` is less than 1, the default of 4 MB is used.
- If the context is canceled during an upload, the request returns the
  context's error right away.

### Reading the response

```go
res.StatusCode()    // int, such as 200
res.Body()          // body as a string
res.Binary()        // body as []byte
res.Err()           // error from reading the body, or nil
res.ContentLength() // int64, -1 if unknown
res.GetHeader().Get("Content-Type")
res.Proto()         // "HTTP/1.1", "HTTP/2.0", ...
res.ProtoMajor()
res.ProtoMinor()
res.Close()         // whether the server asked to close the connection
res.Uncompressed()  // whether the body was decompressed automatically
```

The body is read from the network the first time you call `Body()`,
`Binary()` or `Err()`. Later calls return the same content, so you can call
them as often as you like.

If reading the body fails, `Body()` returns `""` and `Binary()` returns an
empty slice. Call `Err()` to get the error:

```go
body := res.Binary()
if err := res.Err(); err != nil {
	log.Fatal(err)
}
```

> **Note:** Always read the body (`Body()`, `Binary()` or `Err()`), even if you
> don't need it. Reading the body also closes it. A body that is never read
> keeps its connection open.

### Checking the status code

Check the class of the status code:

```go
switch {
case res.IsStatus200s():
	fmt.Println("success")
case res.IsStatus300s():
	fmt.Println("redirect")
case res.IsStatus400s():
	fmt.Println("client error")
case res.IsStatus500s():
	fmt.Println("server error")
}
```

| Method           | Range     |
|------------------|-----------|
| `IsStatus100s()` | 100 – 199 |
| `IsStatus200s()` | 200 – 299 |
| `IsStatus300s()` | 300 – 399 |
| `IsStatus400s()` | 400 – 499 |
| `IsStatus500s()` | 500 – 599 |

Or check an exact code:

```go
if res.IsStatus404() {
	fmt.Println("not found")
}
```

There is an `IsStatusXXX()` method for each status code defined in
`net/http`, from `IsStatus100()` to `IsStatus511()`.

## Error handling

Request methods return an error when:

- the context or the URL is `nil`
- an upload method is called with a `nil` stream
- the request cannot be sent (DNS failure, connection refused, TLS error, and
  so on)
- the context is canceled or its deadline passes
- an upload stream returns an error while it is being read

HTTP error statuses such as 404 or 500 are **not** returned as errors. Check
them with `StatusCode()` or the `IsStatus...()` methods.

## Development

Run the tests:

```sh
go test -race -cover ./...
```

## License

Annette is distributed under the MIT License. See [LICENSE](LICENSE) or https://opensource.org/licenses/mit-license.php.


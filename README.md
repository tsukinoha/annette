# Annette -HTTP Client-

Annette is a simple HTTP client for Go.

It wraps `net/http` with a small API: build a `Client`, call a method such as
`Get(uri)` or `Post(uri, body)`, and read the result through helper methods
like `Body()` and `IsStatus200()`. It can also stream large uploads in chunks
without loading the whole payload into memory.

## Features

- One method per HTTP verb: `Get`, `Head`, `Post`, `Put`, `Patch`, `Delete`,
  `Options` and `Trace`
- Streaming uploads (`UploadByPost` / `UploadByPut` / `UploadByPatch`) with a
  configurable chunk size
- Default headers and a timeout for each client (30 seconds by default)
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
	"fmt"
	"log"
	"net/url"

	"github.com/tsukinoha/annette"
)

func main() {
	uri, err := url.Parse("https://example.com")
	if err != nil {
		log.Fatal(err)
	}

	client := annette.New()
	client.SetTimeout(10) // seconds; optional, the default is 30 seconds

	res, err := client.Get(uri)
	if err != nil {
		log.Fatal(err)
	}

	body := res.Body()
	if err := res.Err(); err != nil {
		log.Fatal(err)
	}

	fmt.Println(res.StatusCode())
	fmt.Println(res.GetHeader().Get("Content-Type"))
	fmt.Println(body)
}
```

## Usage

### Creating a client

`New` creates a client. The URL is not stored in the client; you pass it to
each request, so one client can be shared by many requests (and goroutines) to
different URLs. The client has these public fields:

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

```go
uri, _ := url.Parse("https://api.example.com/items")

res, err := client.Get(uri)
res, err := client.Head(uri)
res, err := client.Delete(uri)
res, err := client.Options(uri)
res, err := client.Trace(uri)

// Methods with a body take an io.Reader.
res, err := client.Post(uri, strings.NewReader(`{"name":"annette"}`))
res, err := client.Put(uri, bytes.NewReader(data))
res, err := client.Patch(uri, strings.NewReader(`{"name":"new"}`))
```

The `Content-Type` header is not set for you. Set it in `client.Header` when
the server needs it:

```go
client.Header.Set("Content-Type", "application/json")
res, err := client.Post(uri, strings.NewReader(`{"name":"annette"}`))
```

### Timeouts

Every request is limited by the client's timeout, which is 30 seconds by
default. Change it with `SetTimeout`, which takes the number of seconds:

```go
client := annette.New()
client.SetTimeout(10)
fmt.Println(client.Timeout()) // 10s (Timeout returns a time.Duration)

client.SetTimeout(0)  // no timeout
client.SetTimeout(-1) // back to the default of 30 seconds
```

- The timeout covers the whole request: connecting, sending the request body,
  waiting for the response and reading the response body with `Body()`,
  `Binary()` or `Err()`.
- When the timeout expires, the request method (or `Err()`, if it expires while
  the body is being read) returns an error that wraps
  `context.DeadlineExceeded`:

  ```go
  res, err := client.Get(uri)
  if errors.Is(err, context.DeadlineExceeded) {
  	log.Println("timed out")
  }
  ```

- `0` means no timeout, and a negative value restores the default of 30
  seconds.
- The timeout is set in whole seconds, so it cannot be shorter than one second
  (except `0`).
- Call `SetTimeout` before you start sending requests. It must not be called
  while requests made with the same client are in progress. To use different
  timeouts, create a client for each one.

### Uploading large files

The upload methods stream data from an `io.ReadCloser` to the server in chunks
of `ChunkSize` bytes, so the whole file is never held in memory. The request is
sent with `Content-Type: application/octet-stream`.

The client's timeout also applies to uploads, and large uploads can take a long
time. Use a separate client for uploads with a longer timeout:

```go
uploader := annette.New()
uploader.SetTimeout(30 * 60) // 30 minutes
uploader.ChunkSize = 1024 * 1024 // 1 MB (optional)

f, err := os.Open(path)
if err != nil {
	return err
}
res, err := uploader.UploadByPut(uri, f) // or UploadByPost / UploadByPatch
if err != nil {
	return err
}
if err := res.Err(); err != nil {
	return err
}
fmt.Println(res.StatusCode())
```

- The client closes the stream for you, including when the request fails.
- If reading the stream fails partway, the request fails as well, so a
  partial upload is not reported as a success.
- If `ChunkSize` is less than 1, the default of 4 MB is used.
- If the timeout expires during an upload, the request returns the error
  right away, even when the stream is blocked. With `SetTimeout(0)` a blocked
  stream is never stopped, so set a timeout for uploads.

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

- the URL is `nil`
- an upload method is called with a `nil` stream
- the request cannot be sent (DNS failure, connection refused, TLS error, and
  so on)
- the client's timeout expires (the error wraps `context.DeadlineExceeded`)
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


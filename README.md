# Annette -HTTP Client-

Annette is a simple HTTP client for Go.

It wraps `net/http` with a small API: build a `Client` for one URL, call a
method such as `Get()` or `Post()`, and read the result through helper methods
like `Body()` and `IsStatus200()`. It can also stream large uploads in chunks
without loading the whole payload into memory.

## Features

- One method per HTTP verb: `Get`, `Head`, `Post`, `Put`, `Patch`, `Delete`,
  `Options` and `Trace`
- Streaming uploads (`UploadByPost` / `UploadByPut` / `UploadByPatch`) with a
  configurable chunk size
- Default headers and a `context.Context` for each client
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

	client := annette.New(uri)
	res, err := client.Get()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(res.StatusCode())
	fmt.Println(res.GetHeader().Get("Content-Type"))
	fmt.Println(res.Body())
}
```

## Usage

### Creating a client

`New` creates a client for one URL. The client has these public fields:

| Field       | Type              | Default                | Description                              |
|-------------|-------------------|------------------------|------------------------------------------|
| `Header`    | `http.Header`     | empty                  | Headers sent with every request          |
| `Context`   | `context.Context` | `context.Background()` | Context used for every request           |
| `ChunkSize` | `int`             | `1048576` (1 MB)       | Buffer size for streaming uploads        |

```go
uri, _ := url.Parse("https://api.example.com/items")
client := annette.New(uri)

client.Header.Set("Authorization", "Bearer <token>")
client.Header.Set("Accept", "application/json")
```

Each request uses a copy of `client.Header`, so a request never changes the
client's headers.

### Sending requests

```go
res, err := client.Get()
res, err := client.Head()
res, err := client.Delete()
res, err := client.Options()
res, err := client.Trace()

// Methods with a body take an io.Reader.
res, err := client.Post(strings.NewReader(`{"name":"annette"}`))
res, err := client.Put(bytes.NewReader(data))
res, err := client.Patch(strings.NewReader(`{"name":"new"}`))
```

The `Content-Type` header is not set for you. Set it in `client.Header` when
the server needs it:

```go
client.Header.Set("Content-Type", "application/json")
res, err := client.Post(strings.NewReader(`{"name":"annette"}`))
```

### Timeouts and cancellation

Set `client.Context` to control how long a request may take:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

client.Context = ctx
res, err := client.Get() // err wraps context.DeadlineExceeded after 10 seconds
```

### Uploading large files

The upload methods stream data from an `io.ReadCloser` to the server in chunks
of `ChunkSize` bytes, so the whole file is never held in memory. The request is
sent with `Content-Type: application/octet-stream`.

```go
f, err := os.Open("large-file.iso")
if err != nil {
	log.Fatal(err)
}

client.ChunkSize = 4 * 1024 * 1024 // 4 MB (optional)
res, err := client.UploadByPut(f)  // or UploadByPost / UploadByPatch
if err != nil {
	log.Fatal(err)
}
fmt.Println(res.StatusCode())
```

- The client closes the stream for you, including when the request fails.
- If reading the stream fails partway, the request fails as well, so a
  partial upload is not reported as a success.
- If `ChunkSize` is less than 1, the default of 1 MB is used.

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

- the client was created with a `nil` URL
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


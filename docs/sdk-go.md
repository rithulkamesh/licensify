# Go SDK

The Go SDK wraps the Licensify C ABI via cgo. It is the best fit when you want an idiomatic Go API but still rely on the shared client core for protocol + crypto + caching.

## Install

Import the module from this repository:

```go
import "github.com/rithulkamesh/licensify/sdk/go/licensify"
```

## Prerequisites

- The Licensify native library must be available to the dynamic loader.
- If you’re developing inside this repo, build the client core first (see the root `README.md`).

## Example

```go
package main

import (
  "fmt"
  "os"
  "github.com/rithulkamesh/licensify/sdk/go/licensify"
)

func main() {
  c, err := licensify.New(licensify.Config{
    ServerURL: "http://localhost:8080",
    CachePath: os.TempDir() + "/licensify.token",
  })
  if err != nil { panic(err) }
  defer c.Close()

  if err := c.Activate("LICENSE-KEY"); err != nil { panic(err) }
  st, err := c.Check()
  if err != nil { panic(err) }
  fmt.Println(st.Code)
}
```

## Errors

Errors are typed:
- `*licensify.InitializationError`
- `*licensify.ActivationError` (includes error code + message)
- `*licensify.CheckError` (includes error code + message)

## Tips

- Store `CachePath` somewhere stable per-user/per-install; `/tmp` is for demos only.
- If you need to differentiate online vs offline checks, look for a “source” field in the returned status (if exposed by your SDK version), otherwise treat it as an implementation detail of the core.

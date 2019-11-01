# bit32 for GopherLua

A native Go implementation of [bit32](https://luarocks.org/modules/siffiejoe/bit32) for the [GopherLua](https://github.com/yuin/gopher-lua) VM.

## Using

### Loading Module

```go
import (
	"github.com/BixData/gluabit32"
)

// Bring up a GopherLua VM
L := lua.NewState()
defer L.Close()

// Preload bit32 module
gluabit32.Preload(L)
```

### Invoking functions

```go
script := `
  local bit32 = require 'bit32'
  return bit32.rshift(5, 1), bit32.rshift(0xffffffff, 0)`
L.DoString(script)
assert.equal(2, L.ToString(-2))
assert.equal(4294967295, L.ToString(-1))
```

## Testing

```bash
$ go test
```

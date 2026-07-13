# sxcli completion — shell completion for sxcli.dev/fw binaries

`sxcli.dev/completion` adds bash/zsh/fish completion to any binary built
on [`sxcli.dev/fw`](https://sxcli.dev). It is deliberately **not** part
of the framework core: it consumes the core's public Introspector API
and registers one *System* applet per shell — proof that the core stays
simple and the introspection surface is sufficient.

Support is linked per shell, exactly like the framework's log sinks:

```go
import (
    _ "sxcli.dev/completion/bash"
    _ "sxcli.dev/completion/zsh"
)
```

Status: design phase. See `docs/specs/` for the living design document.

Guides and demos live at [sxcli.dev](https://sxcli.dev).

## License

Apache-2.0

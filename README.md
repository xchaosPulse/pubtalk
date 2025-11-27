# gemini-audio (prototype)

Small prototype CLI to manage application-to-application audio routing on Ubuntu.

Usage (basic):

```
go run ./main.go status
go run ./main.go --dry-run setup profiles/firefox-to-chrome.yaml
```

This scaffold includes:
- a minimal CLI (`main.go`) with `status`, `setup`, and `reset` placeholders
- an `internal/pactl` package that wraps `pactl` with dry-run support
- an example profile in `profiles/firefox-to-chrome.yaml`

Next steps:
- implement profile parsing and routing application
- add integration tests that run on a Linux machine with PulseAudio / PipeWire

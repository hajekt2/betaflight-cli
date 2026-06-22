# Use go.bug.st/serial for USB serial

USB serial support will use `go.bug.st/serial`.
It is a mature cross-platform Go serial package and fits the single-binary Windows, macOS, and Linux target.
The package should be wrapped behind a narrow internal transport interface so implementation details do not leak into command or protocol code.

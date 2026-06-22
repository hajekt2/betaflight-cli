# Windows, macOS, and Linux are first-class

Windows COM ports, macOS `/dev/tty.*`, Linux `/dev/ttyACM*`, Linux `/dev/ttyUSB*`, and ARM64 builds will be first-class targets from day one.
USB serial is a core workflow, and platform-specific port naming is where cross-platform CLIs often fail late.
Tests, examples, and CI should exercise Windows, macOS, Linux, amd64, and arm64 early.

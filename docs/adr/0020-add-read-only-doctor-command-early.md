# Add read-only doctor command early

`doctor` will be an early read-only diagnostic command.
It should list serial ports, explain auto-detection candidates, optionally test MSP handshakes, report firmware support status, and provide platform-specific hints.
JSON-first diagnostics help agents and humans debug connection failures without touching Flight Controller configuration.
By default, `doctor` lists ports without opening them.
It only sends `MSP_API_VERSION` probes when `--probe` is explicit.

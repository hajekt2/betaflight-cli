# USB serial only initial transport

The initial implementation will support USB serial connections to already-running Betaflight firmware only.
Bluetooth, TCP, UDP, browser-owned WebSerial bridges, and other non-USB transports are out of scope because they add platform behavior that does not help the first AI-agent workflow.
The code may still use an internal transport interface for tests and future extension, but public support starts with USB serial.

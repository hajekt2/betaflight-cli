# Add fake Flight Controller for tests

The project will include fake transports and a minimal fake Flight Controller early.
Captured MSP frame tests are necessary, but command workflow tests need stateful behavior such as handshake, framed CLI responses, timeouts, unsupported commands, and save/reboot disconnects.
A fake Flight Controller lets safety rules and JSON contracts be tested without real hardware.

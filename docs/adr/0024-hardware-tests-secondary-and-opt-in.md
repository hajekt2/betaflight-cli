# Hardware tests are secondary and opt-in

Unit tests, captured-frame tests, fake transport tests, and fake Flight Controller workflow tests are the primary validation suite.
Read-only hardware integration tests may run against a real connected Flight Controller, but they are secondary and must be explicitly enabled with a port and build tag.
Hardware tests must never write, save, reboot, erase, or move motors.

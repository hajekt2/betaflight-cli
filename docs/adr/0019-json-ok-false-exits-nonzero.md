# JSON ok false exits non-zero

When a JSON Response Envelope has `ok: false`, the process will exit non-zero.
This includes safe refusals such as missing confirmation, unsupported firmware, and multiple auto-detected targets.
Agents can distinguish cases through stable `errors[].code` values while shell semantics remain simple.

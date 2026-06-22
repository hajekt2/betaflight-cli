# Support official Betaflight 2025.12 and newer

First-class support will target official Betaflight `2025.12.x` and newer firmware.
Older `4.x` firmware and Betaflight forks are outside the initial support matrix because full Configurator parity already creates a large compatibility surface.
Raw CLI passthrough and raw MSP diagnostics may still work with warnings when the MSP major version is compatible, but domain commands only promise behavior for compiled metadata targets.
Domain commands hard-fail outside the compiled metadata support set unless `--allow-unsupported` is explicit.

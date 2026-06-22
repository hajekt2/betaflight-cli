# Use Cobra for the CLI command surface

`betaflight-cli` will use Cobra for the user-facing command surface.
Full non-graphical Configurator parity implies many nested command families, stable help text, completions, and consistent flag behavior.
Cobra is heavier than tiny parsers, but the command surface is large enough that the structure is worth it.

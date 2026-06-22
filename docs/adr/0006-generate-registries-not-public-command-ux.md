# Generate registries, not public command UX

Low-level MSP codes, setting metadata, and compatibility facts will be generated where practical, but user-facing command families will be hand-written.
Generated metadata gives drift detection, validation, and maintainability across Betaflight releases.
Hand-written domain commands keep the CLI ergonomic and safe, while configuration mutations reuse Betaflight CLI commands instead of inventing a parallel configuration language.

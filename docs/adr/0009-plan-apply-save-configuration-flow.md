# Plan, apply, save configuration flow

Configuration commands will separate planning, applying, and saving.
By default, `settings set name value` returns a structured Change Plan and does not write to the Flight Controller.
`--apply` sends the CLI-backed change without saving, while `--save` persists and usually reboots, so it requires explicit confirmation.
`--apply` never implies `save`.
Batch changes should use plan files or stdin so agents can show the entire proposed diff before applying it.

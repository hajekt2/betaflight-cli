# Non-interactive commands do not prompt

Non-interactive commands will never prompt by default because `betaflight-cli` is AI-agent first and must not hang scripts or tool calls waiting for stdin.
When confirmation is missing, commands return a structured JSON error explaining the required flag or confirmation token.
Human prompts are available only through explicit interactive mode.

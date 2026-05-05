#!/bin/bash
# PostToolUse hook: send truncated tool call info to user as progress
# Receives JSON on stdin from Claude Code hooks system
# Toggle: create ~/.wx-cli/notify/{profile} to enable
[[ ! -f "$HOME/.wx-cli/notify/$WX_PROFILE" ]] && exit 0
[[ -z "$SEND_HELPER" || ! -x "$SEND_HELPER" ]] && exit 0

read -r json
tool=$(echo "$json" | jq -r '.tool_name // empty')
cmd=$(echo "$json" | jq -r '.tool_input.command // empty' | head -c 80)

[[ -z "$tool" || -z "$cmd" ]] && exit 0

bash "$SEND_HELPER" --text "⏳ $cmd" 2>/dev/null &
exit 0

#!/bin/bash
# Wrapper for claude-hud statusLine: caches rate_limits for bot /usage command
# Usage: configure in ~/.claude/settings.json:
#   "statusLine": {
#     "type": "command",
#     "command": "bash ~/.claude/skills/wx-cli/tools/hud_wrapper.sh"
#   }
input=$(cat)

# Cache rate_limits for bot /usage command
echo "$input" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    rl = d.get('rate_limits')
    if rl:
        open('/tmp/claude_rate_limits.json', 'w').write(json.dumps(rl))
    cw = d.get('context_window')
    if cw:
        open('/tmp/claude_context_window.json', 'w').write(json.dumps(cw))
except Exception:
    pass
" 2>/dev/null || true

# Run claude-hud
plugin_dir=$(ls -d "${CLAUDE_CONFIG_DIR:-$HOME/.claude}"/plugins/cache/claude-hud/claude-hud/*/ 2>/dev/null \
  | awk -F/ '{ print $(NF-1) "\t" $0 }' \
  | sort -t. -k1,1n -k2,2n -k3,3n -k4,4n \
  | tail -1 | cut -f2-)

echo "$input" | node "${plugin_dir}dist/index.js"

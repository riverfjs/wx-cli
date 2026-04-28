#!/bin/bash
set -eo pipefail

# Usage: bash wx-bot.sh [profile]
PROFILE="${1:-default}"
PROFILE_FLAG=""
[[ "$PROFILE" != "default" ]] && PROFILE_FLAG="--profile $PROFILE"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
HANDLER="$SCRIPT_DIR/handlers/reply.sh"
WX="$HOME/.claude/skills/wx-cli/bin/wx"

# ── check login ──
if ! $WX $PROFILE_FLAG accounts >/dev/null 2>&1; then
  echo "Not logged in. Run: $WX login${PROFILE:+ --profile $PROFILE}"
  exit 1
fi

echo "=== wx-bot started [$PROFILE] ==="
echo "Listening for WeChat messages, Claude auto-reply"
echo "Ctrl+C to stop"
echo "-------------------------------"

# ── rate limit check ──
check_overload() {
  python3 - <<'PYEOF'
import json, time, sys
CACHE = "/tmp/claude_rate_limits.json"
try:
    with open(CACHE) as f:
        rl = json.load(f)
except Exception:
    sys.exit(0)
fh = rl.get("five_hour", {})
pct = fh.get("used_percentage", 0)
if pct >= 85:
    secs = max(0, int(fh.get("resets_at", 0)) - int(time.time()))
    h, m = divmod(secs // 60, 60)
    print(f"Claude usage at {round(pct)}%, resets in {h}h {m}m")
PYEOF
}

# ── monitor loop ──
$WX $PROFILE_FLAG monitor | while IFS= read -r line; do
  from=$(echo "$line" | jq -r '.from_user_id // empty')
  ctx=$(echo "$line" | jq -r '.context_token // empty')
  type=$(echo "$line" | jq -r '.type // empty')
  text=$(echo "$line" | jq -r '.text // empty')
  ts=$(echo "$line" | jq -r '.time // empty')

  [[ -z "$from" || -z "$ctx" ]] && continue

  echo "[$ts] [$type] ${from:0:8}...: $text"

  # text and voice transcription only
  if [[ "$type" != "text" && "$type" != "voice" ]]; then
    $WX $PROFILE_FLAG send --to "$from" --ctx "$ctx" --text "I can only handle text messages for now." &
    continue
  fi

  [[ -z "$text" ]] && continue

  # built-in commands (no Claude)
  if [[ "$text" == "/help" ]]; then
    $WX $PROFILE_FLAG send --to "$from" --ctx "$ctx" \
      --text "Commands: /help /ping /usage
Or just send a message to chat." &
    continue
  fi
  if [[ "$text" == "/ping" ]]; then
    $WX $PROFILE_FLAG send --to "$from" --ctx "$ctx" --text "pong" &
    continue
  fi
  if [[ "$text" == "/usage" ]]; then
    usage=$(check_overload)
    $WX $PROFILE_FLAG send --to "$from" --ctx "$ctx" --text "${usage:-Claude usage: OK}" &
    continue
  fi

  # rate limit guard
  overload=$(check_overload)
  if [[ -n "$overload" ]]; then
    $WX $PROFILE_FLAG send --to "$from" --ctx "$ctx" --text "$overload" &
    continue
  fi

  # dispatch to handler (fully detached from pipeline)
  bash "$HANDLER" "$PROFILE" "$from" "$ctx" "$text" "$ts" </dev/null &

done

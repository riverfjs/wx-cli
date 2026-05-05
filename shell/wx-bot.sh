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
import json, time, sys, os
CACHE = "/tmp/claude_rate_limits.json"
if not os.path.exists(CACHE):
    sys.exit(0)
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

check_usage() {
  python3 - "$PROFILE" <<'PYEOF'
import json, time, sys, os

def bar(pct, width=10):
    filled = round(pct / 100 * width)
    return "█" * filled + "░" * (width - filled)

def time_until(ts):
    if not ts: return "?"
    secs = max(0, int(ts) - int(time.time()))
    h, rem = divmod(secs, 3600)
    m = rem // 60
    return f"{h//24}d {h%24}h" if h >= 24 else f"{h}h {m}m"

profile = sys.argv[1] if len(sys.argv) > 1 else ""
rl = None
try: rl = json.load(open("/tmp/claude_rate_limits.json"))
except: pass

lines = ["📊 Claude"]
try:
    cw = json.load(open(f"/tmp/claude_context_{profile}.json"))
    pct = cw.get("pct", 0)
    lines.append(f"Context {bar(pct)} {pct}%")
except: pass
if rl:
    fh = rl.get("five_hour", {})
    if fh:
        pct = fh.get("used_percentage", 0)
        lines.append(f"Usage {bar(round(pct))} {round(pct)}% (resets in {time_until(fh.get('resets_at'))})")
    sw = rl.get("seven_day", {})
    if sw:
        pct = sw.get("used_percentage", 0)
        lines.append(f"Weekly {bar(round(pct))} {round(pct)}% (resets in {time_until(sw.get('resets_at'))})")
if len(lines) == 1:
    lines.append("暂无数据")
print("\n".join(lines))
PYEOF
}

# ── monitor loop ──
$WX $PROFILE_FLAG monitor | while IFS= read -r line; do
  from=$(echo "$line" | jq -r '.from_user_id // empty')
  ctx=$(echo "$line" | jq -r '.context_token // empty')
  type=$(echo "$line" | jq -r '.type // empty')
  text=$(echo "$line" | jq -r '.text // empty')
  ts=$(echo "$line" | jq -r '.time // empty')
  image_path=$(echo "$line" | jq -r '.image_path // empty')
  file_path=$(echo "$line" | jq -r '.file_path // empty')
  file_name=$(echo "$line" | jq -r '.file_name // empty')

  [[ -z "$from" || -z "$ctx" ]] && continue

  echo "[$ts] [$type] ${from:0:8}...: $text"

  # unsupported types
  if [[ "$type" == "video" ]]; then
    $WX $PROFILE_FLAG send --to "$from" --ctx "$ctx" --text "I can't handle videos yet." &
    continue
  fi

  # image: need image_path
  if [[ "$type" == "image" && -z "$image_path" ]]; then
    $WX $PROFILE_FLAG send --to "$from" --ctx "$ctx" --text "Failed to download image." &
    continue
  fi

  # file: need file_path
  if [[ "$type" == "file" && -z "$file_path" ]]; then
    $WX $PROFILE_FLAG send --to "$from" --ctx "$ctx" --text "Failed to download file." &
    continue
  fi

  # text/voice need text content (unless image/file)
  [[ "$type" != "image" && "$type" != "file" && -z "$text" ]] && continue

  # built-in commands (no Claude)
  if [[ "$text" == "/help" ]]; then
    $WX $PROFILE_FLAG send --to "$from" --ctx "$ctx" \
      --text "Commands: /help /ping /usage /new
Or just send a message to chat." &
    continue
  fi
  if [[ "$text" == "/new" ]]; then
    session_hash=$(echo -n "${PROFILE}_${from}" | sha256sum | cut -c1-32)
    session_uuid="${session_hash:0:8}-${session_hash:8:4}-${session_hash:12:4}-${session_hash:16:4}-${session_hash:20:12}"
    find ~/.claude/projects/ -name "${session_uuid}.jsonl" -delete 2>/dev/null
    find ~/.claude/projects/ -name "${session_uuid}" -type d -exec rm -rf {} + 2>/dev/null
    $WX $PROFILE_FLAG send --to "$from" --ctx "$ctx" --text "Session cleared." &
    continue
  fi
  if [[ "$text" == "/ping" ]]; then
    $WX $PROFILE_FLAG send --to "$from" --ctx "$ctx" --text "pong" &
    continue
  fi
  if [[ "$text" == "/usage" ]]; then
    usage=$(check_usage)
    $WX $PROFILE_FLAG send --to "$from" --ctx "$ctx" --text "$usage" &
    continue
  fi

  # rate limit guard
  overload=$(check_overload)
  if [[ -n "$overload" ]]; then
    $WX $PROFILE_FLAG send --to "$from" --ctx "$ctx" --text "$overload" &
    continue
  fi

  # dispatch to handler (fully detached from pipeline)
  bash "$HANDLER" "$PROFILE" "$from" "$ctx" "$text" "$ts" "$image_path" "$file_path" "$file_name" "$type" </dev/null &

done

#!/bin/bash
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
WX="$HOME/.claude/skills/wx-cli/bin/wx"
ROLE_TEMPLATE="$PROJECT_DIR/prompts/system_role.md"
HISTORY_DIR="$HOME/.wx-cli/history"
mkdir -p "$HISTORY_DIR"

# ── check login ──
if ! "$WX" accounts >/dev/null 2>&1 || [ -z "$("$WX" accounts 2>/dev/null)" ]; then
  echo "Not logged in. Run: $WX login"
  exit 1
fi

echo "=== wx-bot started ==="
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

# ── build system prompt with actual user_id and context_token ──
build_role() {
  local to="$1" ctx="$2"
  sed -e "s|{to_user_id}|$to|g" -e "s|{context_token}|$ctx|g" "$ROLE_TEMPLATE"
}

# ── per-user history (1 round) ──
# File format: JSON {"q":"...","a":"..."}
load_history() {
  local uid="$1"
  local hfile="$HISTORY_DIR/${uid}.json"
  if [[ -f "$hfile" ]]; then
    local q a
    q=$(jq -r '.q // empty' "$hfile" 2>/dev/null)
    a=$(jq -r '.a // empty' "$hfile" 2>/dev/null)
    if [[ -n "$q" && -n "$a" ]]; then
      printf "[Previous exchange]\nUser: %s\nAssistant: %s\n\n[Current message]\n" "$q" "$a"
      return
    fi
  fi
}

save_history() {
  local uid="$1" question="$2" answer="$3"
  printf '{"q":%s,"a":%s}\n' \
    "$(jq -Rns --arg s "$question" '$s')" \
    "$(jq -Rns --arg s "$answer" '$s')" \
    > "$HISTORY_DIR/${uid}.json"
}

# ── monitor loop ──
"$WX" monitor | while IFS= read -r line; do
  from=$(echo "$line" | jq -r '.from_user_id // empty')
  ctx=$(echo "$line" | jq -r '.context_token // empty')
  type=$(echo "$line" | jq -r '.type // empty')
  text=$(echo "$line" | jq -r '.text // empty')
  ts=$(echo "$line" | jq -r '.time // empty')

  [[ -z "$from" || -z "$ctx" ]] && continue

  echo "[$ts] [$type] ${from:0:8}...: $text"

  # text and voice transcription only
  if [[ "$type" != "text" && "$type" != "voice" ]]; then
    "$WX" send --to "$from" --ctx "$ctx" --text "I can only handle text messages for now." &
    continue
  fi

  [[ -z "$text" ]] && continue

  # built-in commands (no Claude)
  if [[ "$text" == "/help" ]]; then
    "$WX" send --to "$from" --ctx "$ctx" \
      --text "Commands: /help /ping /usage
Or just send a message to chat." &
    continue
  fi
  if [[ "$text" == "/ping" ]]; then
    "$WX" send --to "$from" --ctx "$ctx" --text "pong" &
    continue
  fi
  if [[ "$text" == "/usage" ]]; then
    usage=$(check_overload)
    "$WX" send --to "$from" --ctx "$ctx" --text "${usage:-Claude usage: OK}" &
    continue
  fi

  # rate limit guard
  overload=$(check_overload)
  if [[ -n "$overload" ]]; then
    "$WX" send --to "$from" --ctx "$ctx" --text "$overload" &
    continue
  fi

  # async: claude --print → wx send (with "thinking" timeout)
  (
    role=$(build_role "$from" "$ctx")
    history_prefix=$(load_history "$from")
    prompt="${history_prefix}User: ${text}"

    tmpfile=$(mktemp /tmp/wx-reply-XXXXXX)
    thinking_sent=0

    # background: claude writes reply to tmpfile
    claude --print \
      --model claude-sonnet-4-6 \
      --permission-mode bypassPermissions \
      --append-system-prompt "$role" \
      -p "$prompt" > "$tmpfile" 2>/dev/null &
    claude_pid=$!

    # timeout: if no reply in 10s, send "thinking..."
    (
      sleep 10
      if kill -0 "$claude_pid" 2>/dev/null; then
        "$WX" send --to "$from" --ctx "$ctx" --text "Thinking..." 2>/dev/null
      fi
    ) &
    timer_pid=$!

    wait "$claude_pid" 2>/dev/null
    kill "$timer_pid" 2>/dev/null; wait "$timer_pid" 2>/dev/null

    reply=$(cat "$tmpfile")
    rm -f "$tmpfile"

    if [[ -n "$reply" ]]; then
      "$WX" send --to "$from" --ctx "$ctx" --text "$reply"
      save_history "$from" "$text" "$reply"
    else
      "$WX" send --to "$from" --ctx "$ctx" --text "(Failed to process, please try again later)"
    fi
    echo "[$ts] replied to ${from:0:8}..."
  ) &

done

#!/bin/bash
# Handle a single incoming message: call Claude and reply via wx send.
# Usage: bash reply.sh <profile> <from_user_id> <context_token> <text> <timestamp> [image_path] [file_path] [file_name] [msg_type]
set -eo pipefail

PROFILE="$1"
FROM="$2"
CTX="$3"
TEXT="$4"
TS="$5"
IMAGE_PATH="$6"
FILE_PATH="$7"
FILE_NAME="$8"
MSG_TYPE="$9"

export PATH="$HOME/.local/bin:$PATH"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$(dirname "$SCRIPT_DIR")")"
WX="$HOME/.claude/skills/wx-cli/bin/wx"
ROLE_TEMPLATE="$PROJECT_DIR/prompts/system_role.md"

PF=""
[[ -n "$PROFILE" && "$PROFILE" != "default" ]] && PF="--profile $PROFILE"

# create media send helper (agent calls this, never sees ctx)
SEND_HELPER="/tmp/wx-send-media-$$.sh"
cat > "$SEND_HELPER" << ENDHELPER
#!/bin/bash
out=\$($WX $PF send --to "$FROM" --ctx "$CTX" "\$@" 2>&1)
rc=\$?
if [ \$rc -ne 0 ]; then
  echo "SEND_ERROR: \$out" >&2
  exit \$rc
fi
ENDHELPER
chmod +x "$SEND_HELPER"
export SEND_HELPER

# build system prompt
role=$(sed -e "s|{send_helper}|$SEND_HELPER|g" "$ROLE_TEMPLATE")

# deterministic session UUID per profile+user
SESSION_HASH=$(echo -n "${PROFILE}_${FROM}" | sha256sum | cut -c1-32)
SESSION_UUID="${SESSION_HASH:0:8}-${SESSION_HASH:8:4}-${SESSION_HASH:12:4}-${SESSION_HASH:16:4}-${SESSION_HASH:20:12}"
# first call creates session, subsequent calls resume
if find ~/.claude/projects/ -name "${SESSION_UUID}.jsonl" 2>/dev/null | grep -q .; then
  SESSION_FLAG="--resume $SESSION_UUID"
else
  SESSION_FLAG="--session-id $SESSION_UUID"
fi

# build prompt
prompt=""
if [[ -n "$IMAGE_PATH" && -f "$IMAGE_PATH" ]]; then
  prompt="${prompt}[User sent an image at $IMAGE_PATH — use the Read tool to view it, then respond.]
"
fi
if [[ -n "$FILE_PATH" && -f "$FILE_PATH" ]]; then
  prompt="${prompt}[User sent a file: ${FILE_NAME:-$(basename "$FILE_PATH")} at $FILE_PATH — use the Read tool to view it, then respond.]
"
fi
if [[ -n "$TEXT" ]]; then
  prompt="${prompt}$TEXT"
elif [[ -n "$IMAGE_PATH" ]]; then
  prompt="${prompt}Please analyze this image."
elif [[ -n "$FILE_PATH" ]]; then
  prompt="${prompt}Please analyze this file."
fi

# typing indicator keepalive
(
  while true; do
    $WX $PF typing --to "$FROM" --ctx "$CTX" --status start 2>/dev/null
    sleep 5
  done
) &
tpid=$!

# call claude
CLAUDE_ARGS=(--print --output-format json --model opus --permission-mode bypassPermissions $SESSION_FLAG --append-system-prompt "$role" -p "$prompt")
if [[ -n "$IMAGE_PATH" && -f "$IMAGE_PATH" ]]; then
  CLAUDE_ARGS+=(--add-dir "$(dirname "$IMAGE_PATH")")
fi
if [[ -n "$FILE_PATH" && -f "$FILE_PATH" ]]; then
  CLAUDE_ARGS+=(--add-dir "$(dirname "$FILE_PATH")")
fi
CLAUDE_ERR="$HOME/.wx-cli/logs/claude-err_${PROFILE}.log"
set +e
raw=$(claude "${CLAUDE_ARGS[@]}" </dev/null 2>"$CLAUDE_ERR")
claude_exit=$?
set -e

kill $tpid 2>/dev/null || true; wait $tpid 2>/dev/null || true
$WX $PF typing --to "$FROM" --ctx "$CTX" --status stop 2>/dev/null &

# cleanup temp files
[[ -n "$IMAGE_PATH" && -f "$IMAGE_PATH" ]] && rm -f "$IMAGE_PATH"
[[ -n "$FILE_PATH" && -f "$FILE_PATH" ]] && rm -f "$FILE_PATH"
rm -f "$SEND_HELPER"

reply=$(echo "$raw" | jq -r '.result // empty' 2>/dev/null)

if [[ $claude_exit -ne 0 ]]; then
  echo "$raw" > "$CLAUDE_ERR"
  echo "[$TS] claude failed (exit=$claude_exit, see $CLAUDE_ERR)" >&2
  $WX $PF send --to "$FROM" --ctx "$CTX" --text "(Failed, please try again)"
elif [[ -n "$reply" ]]; then
  $WX $PF send --to "$FROM" --ctx "$CTX" --text "$reply"
  echo "[$TS] replied to ${FROM:0:8}..."
  # cache context per profile + warn if high
  ctx_window=$(echo "$raw" | jq '[.modelUsage[]] | .[0].contextWindow // 0' 2>/dev/null)
  if [[ -n "$ctx_window" && "$ctx_window" -gt 0 ]]; then
    ctx_used=$(echo "$raw" | jq '[.usage.input_tokens, .usage.cache_read_input_tokens, .usage.cache_creation_input_tokens] | add // 0' 2>/dev/null)
    ctx_pct=$(( ctx_used * 100 / ctx_window ))
    echo "{\"used\":$ctx_used,\"window\":$ctx_window,\"pct\":$ctx_pct}" > "/tmp/claude_context_${PROFILE}.json"
    if (( ctx_pct > 80 )); then
      $WX $PF send --to "$FROM" --ctx "$CTX" --text "💡 上下文已用 ${ctx_pct}%，发 /new 可开启新会话" 2>/dev/null &
    fi
  fi
else
  echo "[$TS] claude returned empty" >&2
  $WX $PF send --to "$FROM" --ctx "$CTX" --text "(Failed, please try again)"
fi

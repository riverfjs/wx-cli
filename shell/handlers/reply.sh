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
HISTORY_DIR="$HOME/.wx-cli/history/${PROFILE}"
mkdir -p "$HISTORY_DIR"

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

# load 1-round history
prompt=""
hfile="$HISTORY_DIR/${FROM}.json"
if [[ -f "$hfile" ]]; then
  q=$(jq -r '.q // empty' "$hfile" 2>/dev/null)
  a=$(jq -r '.a // empty' "$hfile" 2>/dev/null)
  if [[ -n "$q" && -n "$a" ]]; then
    prompt="[Previous exchange]
User: $q
Assistant: $a

[Current message]
"
  fi
fi
# build current message with optional context
current=""
if [[ -n "$IMAGE_PATH" && -f "$IMAGE_PATH" ]]; then
  current="${current}[User sent an image at $IMAGE_PATH — use the Read tool to view it, then respond.]
"
fi
if [[ -n "$FILE_PATH" && -f "$FILE_PATH" ]]; then
  current="${current}[User sent a file: ${FILE_NAME:-$(basename "$FILE_PATH")} at $FILE_PATH — use the Read tool to view it, then respond.]
"
fi
if [[ -n "$TEXT" ]]; then
  current="${current}User: $TEXT"
elif [[ -n "$IMAGE_PATH" ]]; then
  current="${current}User: Please analyze this image."
elif [[ -n "$FILE_PATH" ]]; then
  current="${current}User: Please analyze this file."
fi
prompt="${prompt}${current}"

# typing indicator keepalive
(
  while true; do
    $WX $PF typing --to "$FROM" --ctx "$CTX" --status start 2>/dev/null
    sleep 5
  done
) &
tpid=$!

# call claude
CLAUDE_ARGS=(--print --model claude-sonnet-4-6 --permission-mode bypassPermissions --append-system-prompt "$role" -p "$prompt")
if [[ -n "$IMAGE_PATH" && -f "$IMAGE_PATH" ]]; then
  CLAUDE_ARGS+=(--add-dir "$(dirname "$IMAGE_PATH")")
fi
if [[ -n "$FILE_PATH" && -f "$FILE_PATH" ]]; then
  CLAUDE_ARGS+=(--add-dir "$(dirname "$FILE_PATH")")
fi
CLAUDE_ERR="$HOME/.wx-cli/logs/claude-err_${PROFILE}.log"
set +e
reply=$(claude "${CLAUDE_ARGS[@]}" </dev/null 2>"$CLAUDE_ERR")
claude_exit=$?
set -e

kill $tpid 2>/dev/null || true; wait $tpid 2>/dev/null || true
$WX $PF typing --to "$FROM" --ctx "$CTX" --status stop 2>/dev/null &

# cleanup temp files
[[ -n "$IMAGE_PATH" && -f "$IMAGE_PATH" ]] && rm -f "$IMAGE_PATH"
[[ -n "$FILE_PATH" && -f "$FILE_PATH" ]] && rm -f "$FILE_PATH"
rm -f "$SEND_HELPER"

if [[ $claude_exit -ne 0 ]]; then
  echo "$reply" > "$CLAUDE_ERR"
  echo "[$TS] claude failed (exit=$claude_exit, see $CLAUDE_ERR)" >&2
  printf '{"q":%s,"a":%s}\n' \
    "$(jq -Rns --arg s "$TEXT" '$s')" \
    "$(jq -Rns --arg s "[ERROR] $reply" '$s')" > "$hfile"
  $WX $PF send --to "$FROM" --ctx "$CTX" --text "(Failed, please try again)"
elif [[ -n "$reply" ]]; then
  $WX $PF send --to "$FROM" --ctx "$CTX" --text "$reply"
  printf '{"q":%s,"a":%s}\n' \
    "$(jq -Rns --arg s "$TEXT" '$s')" \
    "$(jq -Rns --arg s "$reply" '$s')" > "$hfile"
  echo "[$TS] replied to ${FROM:0:8}..."
else
  echo "[$TS] claude returned empty" >&2
  $WX $PF send --to "$FROM" --ctx "$CTX" --text "(Failed, please try again)"
fi

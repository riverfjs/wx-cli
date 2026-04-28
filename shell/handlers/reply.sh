#!/bin/bash
# Handle a single incoming message: call Claude and reply via wx send.
# Usage: bash reply.sh <profile> <from_user_id> <context_token> <text> <timestamp>
set -eo pipefail

PROFILE="$1"
FROM="$2"
CTX="$3"
TEXT="$4"
TS="$5"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$(dirname "$SCRIPT_DIR")")"
WX="$HOME/.claude/skills/wx-cli/bin/wx"
ROLE_TEMPLATE="$PROJECT_DIR/prompts/system_role.md"
HISTORY_DIR="$HOME/.wx-cli/history/${PROFILE}"
mkdir -p "$HISTORY_DIR"

PF=""
[[ -n "$PROFILE" && "$PROFILE" != "default" ]] && PF="--profile $PROFILE"

# build system prompt
role=$(sed -e "s|{profile}|$PROFILE|g" \
           -e "s|{to_user_id}|$FROM|g" \
           -e "s|{context_token}|$CTX|g" "$ROLE_TEMPLATE")

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
prompt="${prompt}User: $TEXT"

# thinking timer
( sleep 5 && $WX $PF send --to "$FROM" --ctx "$CTX" --text "Thinking..." ) 2>/dev/null &
tpid=$!

# call claude
reply=$(claude --print \
  --model claude-sonnet-4-6 \
  --permission-mode bypassPermissions \
  --append-system-prompt "$role" \
  -p "$prompt" </dev/null 2>/tmp/wx-claude-err.log)

kill $tpid 2>/dev/null || true; wait $tpid 2>/dev/null || true

if [[ -n "$reply" ]]; then
  $WX $PF send --to "$FROM" --ctx "$CTX" --text "$reply"
  printf '{"q":%s,"a":%s}\n' \
    "$(jq -Rns --arg s "$TEXT" '$s')" \
    "$(jq -Rns --arg s "$reply" '$s')" > "$hfile"
  echo "[$TS] replied to ${FROM:0:8}..."
else
  echo "[$TS] claude failed:" >&2
  cat /tmp/wx-claude-err.log >&2
  $WX $PF send --to "$FROM" --ctx "$CTX" --text "(Failed, please try again)"
fi

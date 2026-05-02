#!/bin/bash
set -eo pipefail

SERVE_URL="http://localhost:8080"
WX_TOKEN_FILE="$HOME/.wx-cli/serve/wx_token"

# PUSH_KEY env: per-bot/crontab session token, scoped to one profile
token="${PUSH_KEY:-}"
template=""
text=""
declare -a kv_args

while [[ $# -gt 0 ]]; do
  case "$1" in
    --template) template="$2"; shift 2 ;;
    --text)     text="$2"; shift 2 ;;
    --k[0-9]*)
      num="${1#--k}"
      kv_args+=("-d" "k${num}=$2")
      shift 2 ;;
    *)
      echo '{"ok":false,"error":"unknown arg: '"$1"'"}'; exit 1 ;;
  esac
done

# if no PUSH_KEY, auto-register with serve (crontab scenario)
if [[ -z "$token" ]]; then
  if [[ -z "$WX_PROFILE" ]]; then
    echo '{"ok":false,"error":"no PUSH_KEY and no WX_PROFILE env"}'; exit 1
  fi
  if [[ ! -f "$WX_TOKEN_FILE" ]]; then
    echo '{"ok":false,"error":"wx_token not found"}'; exit 1
  fi
  secret=$(cat "$WX_TOKEN_FILE" | tr -d '[:space:]')
  token=$(curl -sf -X POST "$SERVE_URL/register" \
    -d "profile=$WX_PROFILE" \
    -d "secret=$secret" \
    -d "ttl=10m" 2>/dev/null) || {
    echo '{"ok":false,"error":"register failed, serve running?"}'; exit 1
  }
fi

args=("-sf" "-X" "POST" "$SERVE_URL/push" "-d" "token=$token")

if [[ -n "$template" ]]; then
  args+=("-d" "template=$template" "${kv_args[@]}")
elif [[ -n "$text" ]]; then
  args+=("--data-urlencode" "text=$text")
else
  echo '{"ok":false,"error":"specify --template or --text"}'; exit 1
fi

resp=$(curl "${args[@]}" 2>&1) || {
  echo '{"ok":false,"error":"push failed"}'; exit 1
}

if [[ "$resp" == "ok" ]]; then
  echo '{"ok":true}'
else
  echo '{"ok":false,"error":"'"$resp"'"}'
  exit 1
fi

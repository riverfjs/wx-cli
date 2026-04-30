#!/bin/bash
set -eo pipefail

SERVE_URL="http://localhost:8080/push"
TOKEN_FILE="$HOME/.wx-cli/serve/wx_token"

# WX_PROFILE env var takes priority (set by bot.sh / crontab)
profile="${WX_PROFILE:-}"
template=""
text=""
all=""
declare -a kv_args

while [[ $# -gt 0 ]]; do
  case "$1" in
    --profile)
      if [[ -n "$WX_PROFILE" ]]; then
        echo '{"ok":false,"error":"WX_PROFILE env is set, --profile ignored"}' >&2
      else
        profile="$2"
      fi
      shift 2 ;;
    --all)     all="1"; shift ;;
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

# read wx_token for auth
if [[ ! -f "$TOKEN_FILE" ]]; then
  echo '{"ok":false,"error":"wx_token not found, is serve configured?"}'; exit 1
fi
token=$(cat "$TOKEN_FILE" | tr -d '[:space:]')

if [[ -z "$profile" && -z "$all" ]]; then
  echo '{"ok":false,"error":"no WX_PROFILE env and no --profile specified"}'; exit 1
fi

args=("-sf" "-X" "POST" "$SERVE_URL" "-d" "token=$token")

if [[ -n "$all" ]]; then
  args+=("-d" "all=1")
else
  args+=("-d" "profile=$profile")
fi

if [[ -n "$template" ]]; then
  args+=("-d" "template=$template" "${kv_args[@]}")
elif [[ -n "$text" ]]; then
  args+=("--data-urlencode" "text=$text")
else
  echo '{"ok":false,"error":"specify --template or --text"}'; exit 1
fi

resp=$(curl "${args[@]}" 2>&1) || {
  echo '{"ok":false,"error":"serve unreachable"}'; exit 1
}

if [[ "$resp" == "ok" ]]; then
  echo '{"ok":true}'
else
  echo '{"ok":false,"error":"'"$resp"'"}'
  exit 1
fi

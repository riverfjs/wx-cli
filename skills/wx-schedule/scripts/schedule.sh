#!/bin/bash
set -eo pipefail

SERVE_URL="http://localhost:8080/schedule"
WX_TOKEN_FILE="$HOME/.wx-cli/serve/wx_token"

if [[ ! -f "$WX_TOKEN_FILE" ]]; then
  echo '{"ok":false,"error":"wx_token not found, is serve configured?"}'; exit 1
fi
SECRET=$(cat "$WX_TOKEN_FILE" | tr -d '[:space:]')

ACTION="${1:-}"
[[ -z "$ACTION" ]] && { echo '{"ok":false,"error":"usage: schedule.sh {add|list|del|on|off} [args]"}'; exit 1; }
shift

profile=""
cron_expr=""
command=""
id=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --profile) profile="$2"; shift 2 ;;
    --cron)    cron_expr="$2"; shift 2 ;;
    --cmd)     command="$2"; shift 2 ;;
    --id)      id="$2"; shift 2 ;;
    *) shift ;;
  esac
done

case "$ACTION" in
  add)
    if [[ -z "$profile" || -z "$cron_expr" || -z "$command" ]]; then
      echo '{"ok":false,"error":"add requires --profile, --cron, --cmd"}'; exit 1
    fi
    curl -sf -X POST "$SERVE_URL" \
      -d "secret=$SECRET" \
      -d "action=add" \
      -d "profile=$profile" \
      --data-urlencode "cron=$cron_expr" \
      --data-urlencode "command=$command" 2>/dev/null || {
      echo '{"ok":false,"error":"serve unreachable"}'; exit 1
    }
    ;;
  list)
    args=("secret=$SECRET" "action=list")
    [[ -n "$profile" ]] && args+=("profile=$profile")
    curl -sf -G "$SERVE_URL" $(printf -- '-d %s ' "${args[@]}") 2>/dev/null || {
      echo '{"ok":false,"error":"serve unreachable"}'; exit 1
    }
    ;;
  del)
    [[ -z "$id" ]] && { echo '{"ok":false,"error":"del requires --id"}'; exit 1; }
    curl -sf -X POST "$SERVE_URL" \
      -d "secret=$SECRET" -d "action=del" -d "id=$id" 2>/dev/null || {
      echo '{"ok":false,"error":"serve unreachable"}'; exit 1
    }
    ;;
  on|off)
    [[ -z "$id" ]] && { echo '{"ok":false,"error":"on/off requires --id"}'; exit 1; }
    curl -sf -X POST "$SERVE_URL" \
      -d "secret=$SECRET" -d "action=$ACTION" -d "id=$id" 2>/dev/null || {
      echo '{"ok":false,"error":"serve unreachable"}'; exit 1
    }
    ;;
  *)
    echo '{"ok":false,"error":"unknown action: '"$ACTION"'. use add/list/del/on/off"}'
    exit 1
    ;;
esac

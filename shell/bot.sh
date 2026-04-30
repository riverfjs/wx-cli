#!/bin/bash
set -eo pipefail

WX_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SHELL_DIR="$WX_DIR/shell"
STATE_DIR="$HOME/.wx-cli"
LOG_DIR="$STATE_DIR/logs"
PID_DIR="$STATE_DIR/pids"
mkdir -p "$LOG_DIR" "$PID_DIR"

usage() {
  echo "用法: bash bot.sh {start|stop|status|log} <profile>"
  echo "示例: bash bot.sh start test"
  echo "      bash bot.sh stop test"
  echo "      bash bot.sh status test"
  echo "      bash bot.sh log test"
  exit 1
}

[[ $# -lt 2 ]] && usage

ACTION="$1"
PROFILE="$2"
LOG_FILE="$LOG_DIR/$PROFILE.log"
PID_FILE="$PID_DIR/$PROFILE.pid"

case "$ACTION" in
  start)
    if [[ -f "$PID_FILE" ]] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
      echo "[$PROFILE] 已在运行 (PID $(cat "$PID_FILE"))"
      exit 0
    fi
    # register with serve to get a scoped push token
    WX_TOKEN_FILE="$HOME/.wx-cli/serve/wx_token"
    PUSH_KEY=""
    if [[ -f "$WX_TOKEN_FILE" ]]; then
      PUSH_KEY=$(curl -sf -X POST http://localhost:8080/register \
        -d "profile=$PROFILE" \
        -d "secret=$(cat "$WX_TOKEN_FILE")" 2>/dev/null) || true
    fi
    if [[ -n "$PUSH_KEY" ]]; then
      echo "[$PROFILE] push token 已注册"
    else
      echo "[$PROFILE] push token 注册失败 (serve 未运行?)"
    fi

    # setsid creates a new process group; PID == PGID
    WX_PROFILE="$PROFILE" PUSH_KEY="$PUSH_KEY" setsid bash -c "
      export WX_PROFILE=\"$PROFILE\"
      export PUSH_KEY=\"$PUSH_KEY\"
      bash \"$SHELL_DIR/wx-bot.sh\" \"$PROFILE\"
      if tail -5 \"$LOG_FILE\" | grep -q 'session expired'; then
        echo \"[\$(date +%H:%M:%S)] session expired, notifying serve...\"
        curl -sf -X POST http://localhost:8080/relogin -d \"profile=$PROFILE\" || true
      fi
      rm -f \"$PID_FILE\"
    " >> "$LOG_FILE" 2>&1 &
    echo $! > "$PID_FILE"
    echo "[$PROFILE] 已启动 (PID $!) — 日志: $LOG_FILE"
    ;;
  stop)
    if [[ ! -f "$PID_FILE" ]]; then
      echo "[$PROFILE] 未运行"
      exit 0
    fi
    PGID=$(cat "$PID_FILE")
    kill -- -"$PGID" 2>/dev/null || true
    rm -f "$PID_FILE"
    echo "[$PROFILE] 已停止"
    ;;
  status)
    if [[ -f "$PID_FILE" ]] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
      PID=$(cat "$PID_FILE")
      echo "[$PROFILE] 运行中 (PID $PID)"
    else
      rm -f "$PID_FILE" 2>/dev/null
      echo "[$PROFILE] 未运行"
    fi
    ;;
  log)
    if [[ -f "$LOG_FILE" ]]; then
      tail -50 "$LOG_FILE"
    else
      echo "[$PROFILE] 无日志文件"
    fi
    ;;
  *)
    usage
    ;;
esac

#!/bin/bash
set -eo pipefail

WX_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SHELL_DIR="$WX_DIR/shell"
LOG_DIR="$WX_DIR"

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
PID_FILE="$LOG_DIR/$PROFILE.pid"

case "$ACTION" in
  start)
    if [[ -f "$PID_FILE" ]] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
      echo "[$PROFILE] 已在运行 (PID $(cat "$PID_FILE"))"
      exit 0
    fi
    nohup bash "$SHELL_DIR/wx-bot.sh" "$PROFILE" >> "$LOG_FILE" 2>&1 &
    echo $! > "$PID_FILE"
    echo "[$PROFILE] 已启动 (PID $!) — 日志: $LOG_FILE"
    ;;
  stop)
    if [[ ! -f "$PID_FILE" ]]; then
      echo "[$PROFILE] 未找到 PID 文件，尝试按进程名查杀..."
      pkill -f "wx-bot.sh $PROFILE" 2>/dev/null || true
      pkill -f "wx.*monitor.*$PROFILE" 2>/dev/null || true
      echo "[$PROFILE] 已停止"
      exit 0
    fi
    PID=$(cat "$PID_FILE")
    pkill -P "$PID" 2>/dev/null || true
    kill "$PID" 2>/dev/null || true
    pkill -f "wx-bot.sh $PROFILE" 2>/dev/null || true
    pkill -f "wx.*monitor.*$PROFILE" 2>/dev/null || true
    rm -f "$PID_FILE"
    echo "[$PROFILE] 已停止"
    ;;
  status)
    if [[ -f "$PID_FILE" ]] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
      PID=$(cat "$PID_FILE")
      echo "[$PROFILE] 运行中 (PID $PID)"
      # 显示子进程
      children=$(pgrep -P "$PID" 2>/dev/null | tr '\n' ' ')
      [[ -n "$children" ]] && echo "  子进程: $children"
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

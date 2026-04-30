#!/bin/bash
set -eo pipefail

WX_DIR="$(cd "$(dirname "$0")/.." && pwd)"
WX="$WX_DIR/bin/wx"
NGROK="$HOME/.local/bin/ngrok"
STATE_DIR="$HOME/.wx-cli"
LOG_DIR="$STATE_DIR/logs"
PID_DIR="$STATE_DIR/pids"
SERVE_DIR="$STATE_DIR/serve"
SERVE_LOG="$LOG_DIR/serve.log"
SERVE_PID="$PID_DIR/serve.pid"
NGROK_LOG="$LOG_DIR/ngrok.log"
NGROK_PID="$PID_DIR/ngrok.pid"
TOKEN_FILE="$SERVE_DIR/wx_token"
mkdir -p "$LOG_DIR" "$PID_DIR" "$SERVE_DIR"
PORT="${PORT:-8080}"

usage() {
  echo "用法: bash serve.sh {start|stop|status|url|log|restart-bots|stop-bots} [--port PORT]"
  echo ""
  echo "环境变量:"
  echo "  WX_APPID   微信测试号 AppID (异步回复需要)"
  echo "  WX_SECRET  微信测试号 Secret (异步回复需要)"
  echo "  WX_ROOT    Root 用户 OpenID (模板管理权限)"
  echo "  PORT       监听端口 (默认 8080)"
  echo ""
  echo "示例:"
  echo "  bash serve.sh start"
  echo "  bash serve.sh url    # 查看 ngrok 公网地址"
  echo "  bash serve.sh stop"
  exit 1
}

ensure_token() {
  if [[ -n "$WX_TOKEN" ]]; then
    echo "$WX_TOKEN" > "$TOKEN_FILE"
    return
  fi
  if [[ -f "$TOKEN_FILE" ]]; then
    WX_TOKEN=$(cat "$TOKEN_FILE")
    return
  fi
  WX_TOKEN=$(head -c 16 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 20)
  echo "$WX_TOKEN" > "$TOKEN_FILE"
  echo "[token] 已生成并保存到 $TOKEN_FILE"
}

install_ngrok() {
  if [[ -x "$NGROK" ]]; then
    return
  fi
  echo "安装 ngrok 到 $NGROK ..."
  mkdir -p "$(dirname "$NGROK")"
  local arch
  arch=$(uname -m)
  case "$arch" in
    x86_64)  arch="amd64" ;;
    aarch64) arch="arm64" ;;
  esac
  local url="https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-linux-${arch}.tgz"
  curl -sL "$url" | tar xz -C "$(dirname "$NGROK")"
  chmod +x "$NGROK"
  echo "ngrok 已安装: $NGROK"
}

NO_NGROK=""
parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --port) PORT="$2"; shift 2 ;;
      --no-ngrok) NO_NGROK=1; shift ;;
      *) shift ;;
    esac
  done
}

[[ $# -lt 1 ]] && usage
ACTION="$1"; shift
parse_args "$@"

case "$ACTION" in
  start)
    # check wx binary
    if [[ ! -x "$WX" ]]; then
      echo "wx 未构建，先执行: bash shell/build.sh"
      exit 1
    fi

    ensure_token
    echo "[token] WX_TOKEN=$WX_TOKEN (填入测试号接口配置)"

    # stop existing
    if [[ -f "$SERVE_PID" ]] && kill -0 "$(cat "$SERVE_PID")" 2>/dev/null; then
      echo "serve 已在运行 (PID $(cat "$SERVE_PID"))，先停止..."
      bash "$0" stop
    fi

    # start wx serve
    SERVE_ARGS="serve --port $PORT --wx-token $WX_TOKEN"
    [[ -n "$WX_APPID" ]] && SERVE_ARGS="$SERVE_ARGS --wx-appid $WX_APPID"
    [[ -n "$WX_SECRET" ]] && SERVE_ARGS="$SERVE_ARGS --wx-secret $WX_SECRET"
    [[ -n "$WX_ROOT" ]] && SERVE_ARGS="$SERVE_ARGS --wx-root $WX_ROOT"
    nohup $WX $SERVE_ARGS >> "$SERVE_LOG" 2>&1 &
    echo $! > "$SERVE_PID"
    echo "[serve] 已启动 (PID $!, port $PORT) — 日志: $SERVE_LOG"

    # wait for serve to be ready
    sleep 1

    # auto-start bots from mapping
    MAPPING="$SERVE_DIR/mapping.json"
    if [[ -f "$MAPPING" ]]; then
      BOT_SH="$WX_DIR/shell/bot.sh"
      for alias in $(python3 -c "import json; print(' '.join(json.load(open('$MAPPING')).keys()))" 2>/dev/null); do
        bash "$BOT_SH" start "$alias" 2>&1
      done
    fi

    if [[ -n "$NO_NGROK" ]]; then
      echo "[ngrok] 已跳过 (--no-ngrok)"
      echo "公网地址请自行配置，测试号 URL 填: http://你的IP:$PORT"
    else
      # install & check ngrok authtoken
      install_ngrok
      if ! grep -q 'authtoken' "$HOME/.config/ngrok/ngrok.yml" 2>/dev/null && ! grep -q 'authtoken' "$HOME/.ngrok2/ngrok.yml" 2>/dev/null; then
        echo ""
        echo "[ngrok] 未配置 authtoken，请先执行:"
        echo "  1. 注册: https://dashboard.ngrok.com/signup"
        echo "  2. 配置: $NGROK config add-authtoken <你的TOKEN>"
        echo "  3. 重新启动: bash shell/serve.sh start"
        echo ""
        echo "[serve] 已启动，但 ngrok 未启动（无公网地址）"
        exit 0
      fi

      nohup "$NGROK" http "$PORT" --log=stdout --log-format=logfmt > "$NGROK_LOG" 2>&1 &
      echo $! > "$NGROK_PID"
      echo "[ngrok] 已启动 (PID $!)"

      # wait for tunnel, check if ngrok actually connected
      sleep 3
      if ! kill -0 "$(cat "$NGROK_PID")" 2>/dev/null; then
        echo "[ngrok] 启动失败，查看日志: bash shell/serve.sh log"
        exit 1
      fi
      bash "$0" url
    fi
    ;;

  stop)
    # stop all bots first
    BOT_SH="$WX_DIR/shell/bot.sh"
    for f in "$PID_DIR"/*.pid; do
      [[ ! -f "$f" ]] && continue
      name=$(basename "$f" .pid)
      [[ "$name" == "serve" || "$name" == "ngrok" ]] && continue
      bash "$BOT_SH" stop "$name" 2>&1
    done
    # stop serve + ngrok
    for name in serve ngrok; do
      pid_file="$PID_DIR/${name}.pid"
      if [[ -f "$pid_file" ]]; then
        pid=$(cat "$pid_file")
        kill "$pid" 2>/dev/null || true
        rm -f "$pid_file"
      fi
    done
    pkill -f "bin/wx serve" 2>/dev/null || true
    pkill -f "ngrok http" 2>/dev/null || true
    echo "[serve + ngrok] 已停止"
    ;;

  status)
    for name in serve ngrok; do
      pid_file="$PID_DIR/${name}.pid"
      if [[ -f "$pid_file" ]] && kill -0 "$(cat "$pid_file")" 2>/dev/null; then
        echo "[$name] 运行中 (PID $(cat "$pid_file"))"
      else
        rm -f "$pid_file" 2>/dev/null
        echo "[$name] 未运行"
      fi
    done
    ;;

  url)
    url=$(curl -s http://127.0.0.1:4040/api/tunnels 2>/dev/null \
      | grep -o '"public_url":"https://[^"]*"' \
      | head -1 \
      | cut -d'"' -f4)
    if [[ -n "$url" ]]; then
      echo "公网地址: $url"
      echo "填入测试号接口配置 URL: $url"
    else
      echo "ngrok 隧道未就绪，稍后重试: bash serve.sh url"
    fi
    ;;

  log)
    echo "=== serve ==="
    tail -20 "$SERVE_LOG" 2>/dev/null || echo "(无日志)"
    echo ""
    echo "=== ngrok ==="
    tail -10 "$NGROK_LOG" 2>/dev/null || echo "(无日志)"
    ;;

  restart-bots)
    BOT_SH="$WX_DIR/shell/bot.sh"
    count=0
    for f in "$PID_DIR"/*.pid; do
      [[ ! -f "$f" ]] && continue
      name=$(basename "$f" .pid)
      [[ "$name" == "serve" || "$name" == "ngrok" ]] && continue
      bash "$BOT_SH" stop "$name" 2>&1
      bash "$BOT_SH" start "$name" 2>&1
      ((count++))
    done
    echo "重启完成: $count 个 bot"
    ;;

  stop-bots)
    BOT_SH="$WX_DIR/shell/bot.sh"
    count=0
    for f in "$PID_DIR"/*.pid; do
      [[ ! -f "$f" ]] && continue
      name=$(basename "$f" .pid)
      [[ "$name" == "serve" || "$name" == "ngrok" ]] && continue
      bash "$BOT_SH" stop "$name" 2>&1
      ((count++))
    done
    echo "已停止: $count 个 bot"
    ;;

  *)
    usage
    ;;
esac

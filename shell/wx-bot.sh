#!/bin/bash
set -eo pipefail

WX="$HOME/.claude/skills/wx-cli/bin/wx"

# ── 检查登录 ──
if ! "$WX" accounts >/dev/null 2>&1 || [ -z "$("$WX" accounts 2>/dev/null)" ]; then
  echo "未登录，请先执行: $WX login"
  exit 1
fi

echo "=== wx-bot 启动 ==="
echo "监听微信消息，Claude 自动回复"
echo "按 Ctrl+C 停止"
echo "-------------------------------"

# ── 用量检查 ──
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
    print(f"⚠️ Claude 用量已达 {round(pct)}%，请 {h}h {m}m 后再试")
PYEOF
}

# ── 常驻监听 ──
"$WX" monitor | while IFS= read -r line; do
  from=$(echo "$line" | jq -r '.from_user_id // empty')
  ctx=$(echo "$line" | jq -r '.context_token // empty')
  type=$(echo "$line" | jq -r '.type // empty')
  text=$(echo "$line" | jq -r '.text // empty')
  ts=$(echo "$line" | jq -r '.time // empty')

  [[ -z "$from" || -z "$ctx" ]] && continue

  echo "[$ts] 收到 [$type] from ${from:0:8}...: $text"

  # 只处理文本和语音转文字
  if [[ "$type" != "text" && "$type" != "voice" ]]; then
    "$WX" send --to "$from" --ctx "$ctx" --text "暂时只能处理文字消息哦" &
    continue
  fi

  [[ -z "$text" ]] && continue

  # /help 命令
  if [[ "$text" == "/help" ]]; then
    "$WX" send --to "$from" --ctx "$ctx" \
      --text "👋 我是 AI 助手

直接发消息即可对话
发送 /help 查看帮助" &
    continue
  fi

  # 用量检查
  overload=$(check_overload)
  if [[ -n "$overload" ]]; then
    "$WX" send --to "$from" --ctx "$ctx" --text "$overload" &
    continue
  fi

  # 后台调 Claude 处理
  (
    reply=$(claude --print \
      --model claude-sonnet-4-6 \
      --permission-mode bypassPermissions \
      -p "$text" 2>/dev/null)

    if [[ -n "$reply" ]]; then
      "$WX" send --to "$from" --ctx "$ctx" --text "$reply"
    else
      "$WX" send --to "$from" --ctx "$ctx" --text "（处理失败，请稍后重试）"
    fi
    echo "[$ts] 回复完成: ${from:0:8}..."
  ) &

done

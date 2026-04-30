#!/bin/bash
set -eo pipefail

# 配置 claude-hud + rate_limits 缓存
# 前提: claude-hud 插件已安装 (claude /install-plugin claude-hud)

SETTINGS="$HOME/.claude/settings.json"
WRAPPER="$HOME/.claude/skills/wx-cli/tools/hud_wrapper.sh"

if [[ ! -f "$SETTINGS" ]]; then
  echo "未找到 $SETTINGS"
  exit 1
fi

if ! grep -q "claude-hud" "$SETTINGS" 2>/dev/null; then
  echo "claude-hud 插件未安装"
  echo "请先在 Claude Code 中执行: /install-plugin claude-hud"
  exit 1
fi

# 检查是否已配置
if grep -q "hud_wrapper.sh" "$SETTINGS" 2>/dev/null; then
  echo "已配置，无需重复设置"
  echo "当前配置:"
  grep -A 3 "statusLine" "$SETTINGS"
  exit 0
fi

# 用 python 更新 JSON (保持格式)
python3 -c "
import json
with open('$SETTINGS') as f:
    cfg = json.load(f)
cfg['statusLine'] = {
    'type': 'command',
    'command': 'bash $WRAPPER'
}
with open('$SETTINGS', 'w') as f:
    json.dump(cfg, f, indent=2, ensure_ascii=False)
    f.write('\n')
"

echo "已配置 statusLine -> hud_wrapper.sh"
echo ""
echo "功能:"
echo "  1. 状态栏显示 claude-hud 信息"
echo "  2. 自动缓存 rate_limits 到 /tmp/claude_rate_limits.json"
echo "  3. bot 的 /usage 命令可查询 Claude 用量 (仅订阅模式)"
echo ""
echo "重启 Claude Code 生效"

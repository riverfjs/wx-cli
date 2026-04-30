---
name: wx-push
description: Push notifications to WeChat users via template or plain text messages. Use when sending signal alerts, system notifications, status updates, or any programmatic message to a WeChat user through the wx-cli serve webhook.
---

# wx-push — WeChat Push Notifications

## Goal

Send push notifications to WeChat users via the wx-cli serve webhook server (`localhost:8080`). Supports template messages (structured fields) and plain text (customer service API).

## Hard Constraints

- Always ensure wx serve is running before pushing. Start with `bash ~/.claude/skills/wx-cli/shell/serve.sh start` if needed.
- Always use `bash ~/.claude/skills/wx-push/scripts/push.sh` to send messages. Never call the HTTP endpoint directly.
- Always set `WX_PROFILE` env var to identify the push target. push.sh reads it automatically.
- Never use `--profile` flag when `WX_PROFILE` is set — push.sh ignores it to prevent misrouting.
- Always register templates via WeChat 公众号 root command (`模板 add <name> <id>`) before using `--template`.
- Always check the JSON output for `"ok":true` to confirm delivery.

## Identity Model

| Context | WX_PROFILE source | Who receives push |
|---|---|---|
| Bot process | bot.sh exports it | The bot's own user |
| Crontab | Command prefix `WX_PROFILE=xxx` | The specified user |
| Root manual | Not set, use `--profile` or `--all` | Any user or all users |

push.sh enforces: if `WX_PROFILE` is set, `--profile` is ignored. This prevents bots from accidentally pushing to the wrong user.

## Workflow

### Template push (from bot or crontab)

```bash
# WX_PROFILE is inherited from bot.sh or set in crontab
bash ~/.claude/skills/wx-push/scripts/push.sh \
  --template signal \
  --k1 "AAPL.US" \
  --k2 "买入信号" \
  --k3 "策略B: KDJ金叉确认" \
  --k4 "2026-04-30 14:30:00"
```

### Plain text push

```bash
bash ~/.claude/skills/wx-push/scripts/push.sh --text "服务器告警: disk usage 95%"
```

### Root: push to specific user or broadcast

```bash
# push to one user (only works when WX_PROFILE is NOT set)
bash ~/.claude/skills/wx-push/scripts/push.sh --profile oiNG73xxx --text "notice"

# broadcast to all serve-managed profiles
bash ~/.claude/skills/wx-push/scripts/push.sh --all --text "系统维护通知"
```

## Auth

push.sh reads `~/.wx-cli/serve/wx_token` and sends it with every request. The serve `/push` endpoint validates this token.

## Registered Templates

| Name | Fields | Description |
|------|--------|-------------|
| `signal` | k1=股票, k2=信号, k3=策略, k4=时间 | Trading signal alerts |

## Output

```json
{"ok":true}
{"ok":false,"error":"invalid token"}
```

## When NOT to use this skill

- User wants to send messages as the WeChat bot (use wx-cli send instead).
- User wants interactive WeChat conversations (use wx-cli interactive mode).
- wx serve is not running and user has no intent to start it.

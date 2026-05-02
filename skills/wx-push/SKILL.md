---
name: wx-push
description: Push notifications to WeChat users via template or plain text messages. Use when sending signal alerts, system notifications, status updates, or any programmatic message to a WeChat user through the wx-cli serve webhook.
---

# wx-push — WeChat Push Notifications

## Goal

Send push notifications to WeChat users via the wx-cli serve webhook server (`localhost:8080`). Supports template messages (structured fields) and plain text (customer service API).

## Hard Constraints

- Always ensure wx serve is running before pushing.
- Always use `bash ~/.claude/skills/wx-push/scripts/push.sh` to send messages. Never call the HTTP endpoint directly.
- Always set `WX_PROFILE` env var or ensure `PUSH_KEY` is inherited from bot.sh.
- Always register templates via WeChat 公众号 root command (`模板 add <name> <id>`) before using `--template`.
- Always check the JSON output for `"ok":true` to confirm delivery.
- Never attempt to push to other users. Each PUSH_KEY is scoped to one profile by the server.

## Identity Model

| Context | Auth | How it works |
|---|---|---|
| Bot process | `PUSH_KEY` env (inherited from bot.sh) | Server maps token → profile in memory |
| Crontab | `WX_PROFILE` env | push.sh auto-registers temp token (10min TTL) |
| Root broadcast | WeChat command "广播 xxx" | Only via 公众号, not /push API |

push.sh has no `--profile` or `--all` flag. Target is always determined server-side.

## Workflow

### Template push

```bash
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

## Registered Templates

| Name | Fields | Description |
|------|--------|-------------|
| `signal` | k1=股票, k2=信号, k3=策略, k4=时间 | Trading signal alerts |

## Output

```json
{"ok":true}
{"ok":false,"error":"invalid or expired token"}
```

## When NOT to use this skill

- User wants to send messages as the WeChat bot (use wx-cli send instead).
- User wants interactive WeChat conversations (use wx-cli interactive mode).
- User wants to broadcast to all users (use WeChat root command "广播").
- wx serve is not running and user has no intent to start it.

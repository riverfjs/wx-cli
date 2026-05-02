---
name: wx-cli
description: Send and receive WeChat messages, images, files via the iLink Bot API CLI. Use when user wants to send a WeChat message, image, file, or start the WeChat bot listener.
---

# wx-cli — WeChat iLink Bot CLI

## Goal

Send and receive WeChat personal messages through the Tencent iLink Bot API using the standalone binary at `~/.claude/skills/wx-cli/bin/wx`.

## Hard Constraints

- Always invoke via `~/.claude/skills/wx-cli/bin/wx`. Never reference any source directory.
- Always tell the user to run the CLI themselves via `!` prefix. The CLI is interactive.
- Always include `--profile NAME` when the user has multiple accounts.
- Never send messages without a valid login. Run `wx login` first if `~/.wx-cli/accounts/` is empty.
- Never output bot_token in conversation.

## Workflow

### Login

Tell the user:
```
! ~/.claude/skills/wx-cli/bin/wx login --profile NAME
```
User scans QR code with WeChat and confirms. Token saved to `~/.wx-cli/accounts/{NAME}.json`.

### Start interactive mode

```
! ~/.claude/skills/wx-cli/bin/wx --profile NAME
```

### Start bot daemon

```
! bash ~/.claude/skills/wx-cli/shell/wx-bot.sh NAME
```

### Interactive commands

| Command | Action |
|---------|--------|
| *(plain text)* | Reply to active contact |
| `/contacts` | List known contacts |
| `/use <index\|name>` | Switch active contact |
| `/name <display_name>` | Set nickname for active contact |
| `/image <path>` | Send image (jpg/png/gif/webp) |
| `/file <path>` | Send file attachment |
| `/video <path>` | Send video (mp4) |
| `/ping` | Liveness check |
| `/usage` | Claude rate limit status |
| `/status` | Show bot ID, base URL, login time |
| `/login` | Re-authenticate with new QR code |
| `/quit` | Exit |

### Send a file on behalf of user

1. Confirm the file path exists.
2. Tell the user to start interactive mode and type `/image <path>` or `/file <path>`.
3. TTS voice: `bash ~/.claude/skills/wx-cli/shell/handlers/tts.sh "text"` generates an MP3, then send via `/file`.

### Re-login

Token expires after ~24 hours. Tell the user:
```
! ~/.claude/skills/wx-cli/bin/wx login --profile NAME
```

### List accounts

```bash
~/.claude/skills/wx-cli/bin/wx accounts
```

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `QR code expired` | Re-run login |
| Monitor errors after ~24h | Re-login (token expired) |
| No active contact | Wait for someone to message first |

## When NOT to use this skill

- User wants WeChat mini-programs, official accounts, or enterprise WeChat (WeCom).
- User wants WeChat Pay.
- User needs group chat (iLink API is 1:1 personal messages only).

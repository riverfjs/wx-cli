<p align="center">
  <img src="claude-jumping.svg" alt="Built with Claude Code" width="140">
</p>

# wx-cli

WeChat personal account bot CLI, built on Tencent's official [iLink Bot API](https://github.com/Tencent/openclaw-weixin).

## Features

- QR code login with named profiles (no app ID/secret needed)
- Send & receive text, image, file, video (AES-128-ECB encrypted CDN upload)
- Interactive REPL with contact management
- Daemon mode: `monitor` (JSON lines) + `send` (non-interactive)
- Claude auto-reply bot with conversation history and thinking feedback
- WeChat webhook server: remote login via 公众号, session-expire auto-notify
- Multi-profile support, process management scripts
- 6.6MB standalone Go binary

## Quick start

```bash
git clone <repo-url> ~/.claude/skills/wx-cli
cd ~/.claude/skills/wx-cli
bash shell/build.sh                        # compile bin/wx
cp -r wx-push ~/.claude/skills/wx-push     # install wx-push skill
bin/wx --profile mybot login               # QR scan login
bash shell/bot.sh start mybot              # start Claude auto-reply bot
```

## CLI usage

### Login & accounts

```bash
bin/wx --profile work login                # QR login, save as "work"
bin/wx accounts                            # list all profiles
```

`--profile` is required. Token saved to `~/.wx-cli/accounts/{profile}.json`.

### Interactive REPL

```bash
bin/wx --profile work
```

| Command | Action |
|---------|--------|
| *(text)* | Reply to active contact |
| `/contacts` | List contacts |
| `/use <index>` | Switch active contact |
| `/name <name>` | Set display name |
| `/image <path>` | Send image |
| `/file <path>` | Send file |
| `/video <path>` | Send video |
| `/status` | Connection info |
| `/login` | Re-authenticate |
| `/quit` | Exit |

### Daemon mode

```bash
# monitor: JSON lines per incoming message
bin/wx --profile work monitor

# send: non-interactive
bin/wx --profile work send --to USER_ID --ctx CONTEXT_TOKEN --text "hello"
bin/wx --profile work send --to USER_ID --ctx CONTEXT_TOKEN --image /path/to/pic.png
bin/wx --profile work send --to USER_ID --ctx CONTEXT_TOKEN --file /path/to/doc.pdf
```

## Bot (Claude auto-reply)

```bash
bash shell/wx-bot.sh work                  # foreground
bash shell/bot.sh start work               # background (managed)
bash shell/bot.sh stop work
bash shell/bot.sh status work
bash shell/bot.sh log work
```

PID: `~/.wx-cli/{profile}.pid`, logs: `~/.wx-cli/logs/{profile}.log`

Flow: `wx monitor` → parse JSON → `shell/handlers/reply.sh` → `claude --print` → `wx send`

**Built-in commands** (no Claude):

| Command | Response |
|---------|----------|
| `/ping` | `pong` |
| `/help` | Command list |
| `/usage` | Claude rate limit status |

**Behavior:**
- Text and voice transcription only; other types get a fallback reply
- Per-user conversation history injected as context
- System prompt (`prompts/system_role.md`) with `{profile}`, `{to_user_id}`, `{context_token}` substitution
- 5s thinking timeout sends "Thinking..." feedback
- Rate limit guard at 85% Claude usage

## WeChat webhook server

通过微信公众号测试号实现远程自助登录和断线重连，无需登录服务器。

### Setup

```bash
# 1. 启动 (首次自动安装 ngrok、生成 WX_TOKEN)
WX_APPID=你的appid WX_SECRET=你的secret bash shell/serve.sh start

# 2. 配置测试号
#    打开: https://mp.weixin.qq.com/debug/cgi-bin/sandboxinfo?action=showinfo&t=sandbox/index
#    URL:   bash shell/serve.sh url 输出的地址
#    Token: cat ~/.wx-cli/serve/wx_token
```

> ngrok 免费版每次重启 URL 会变，需重新到测试号页面更新。Token 不变。

### Commands (微信对话)

| 命令 | 权限 | 功能 |
|------|------|------|
| `登录` | 所有人 | 获取 iLink 登录链接，授权后自动启动 bot |
| `状态` | 所有人 | 查看登录状态 |
| `帮助` | 所有人 | 显示命令列表 |
| `服务` | root | 服务运行状态 |
| `活跃` | root | 活跃 Bot 列表 |
| `历史` | root | 最新聊天记录 |
| `重启` | root | 重启所有 Bot |
| `关闭` | root | 停止所有 Bot |
| `模板 add/list/del` | root | 推送模板管理 |

每个用户的 OpenID 自动作为 profile，互相隔离。root 用户通过 `WX_ROOT` 环境变量指定。

### Session-expire auto-reconnect

Bot 检测到会话过期（code -14）时自动通知 serve，serve 向用户推送新登录链接。用户点击重新授权后 bot 自动重启。

serve 只管理通过公众号登录的 profile。serve 重启不影响已运行的 bot。

### Push notifications

通过 `wx-push` skill 推送消息（模板或纯文本）。身份由 `WX_PROFILE` 环境变量决定，bot 进程自动继承，crontab 命令前缀设置。

```bash
# bot 进程内（WX_PROFILE 自动继承）
bash ~/.claude/skills/wx-push/scripts/push.sh --template signal --k1 "AAPL" --k2 "买入" --k3 "策略B" --k4 "14:30"

# crontab（手动设 WX_PROFILE）
WX_PROFILE=oiNG73xxx python3 ~/.claude/skills/signal-monitor/scripts/scan.py AAPL.US --notify push

# root 广播
bash ~/.claude/skills/wx-push/scripts/push.sh --all --text "系统维护通知"
```

安装 push skill: `cp -r wx-push ~/.claude/skills/wx-push`

### Management

```bash
bash shell/serve.sh start          # 启动 serve + ngrok
bash shell/serve.sh stop           # 停止 serve + ngrok
bash shell/serve.sh status         # 运行状态
bash shell/serve.sh url            # 当前公网地址
bash shell/serve.sh log            # 查看日志
bash shell/serve.sh restart-bots   # 重启所有 bot
bash shell/serve.sh stop-bots      # 停止所有 bot
```

## Architecture

```
cmd/wx/main.go         Entry point
internal/
  api/                 Protocol types + HTTP client
  auth/                QR login + token persistence + multi-profile
  cdn/                 AES-128-ECB encrypt + CDN upload
  cli/                 Interactive REPL
  msg/                 Send text/image/file/video + monitor loop
  serve/               WeChat webhook server
    serve.go           Config + HTTP routing
    handler.go         Command dispatch + login/relogin/push flow
    wechat.go          XML types, signature, passive/async/template reply
    bot.go             Bot process management + serve profile/template persistence
    state.go           Process/bot status + chat history queries
shell/
  build.sh             Build script
  bot.sh               Bot process management (start/stop/status/log)
  serve.sh             Serve + ngrok management
  wx-bot.sh            Claude auto-reply daemon
  handlers/reply.sh    Per-message handler
wx-push/               Push notification skill (install to ~/.claude/skills/wx-push)
  SKILL.md             Skill definition
  scripts/push.sh      Push wrapper (WX_PROFILE env + wx_token auth)
prompts/
  system_role.md       System prompt template
```

## Data layout

```
~/.wx-cli/
  accounts/{profile}.json       Credentials
  sync/{profile}                Long-poll cursor
  tokens/{profile}/{uid}        Context token per user
  history/{profile}/{uid}.json  Conversation history per user
  pids/                         All PID files
  logs/                         All log files
  serve/
    wx_token                    WeChat webhook token (auto-generated)
    templates.json              Registered push templates
    profiles                    Serve-managed profiles (one OpenID per line)
```

## Protocol notes

- API base: `https://ilinkai.weixin.qq.com`
- Success response: `{}` (no `ret` field). Error: `{"ret": -2, "errmsg": "..."}`
- Send wrapper: `{"msg": {WeixinMessage}}`, `from_user_id` must be `""`
- Media `aes_key`: `base64(utf8_bytes_of_hex_key)` — not `base64(raw_bytes)`
- Token valid ~24h, re-login required after expiry

## License

MIT

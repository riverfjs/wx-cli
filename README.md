<p align="center">
  <img src="claude-jumping.svg" alt="Built with Claude Code" width="140">
</p>

# wx-cli

[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/lang-Go-00ADD8.svg)](https://go.dev)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS-lightgrey.svg)]()

基于腾讯 [iLink Bot API](https://github.com/Tencent/openclaw-weixin) 的微信个人号机器人 CLI — 多 profile 登录、收发消息、Claude 自动回复、公众号远程登录与消息推送。

* * *

## 功能

- 扫码登录，支持多 profile（无需 appID/secret）
- 收发文本、图片、文件、视频、语音（AES-128-ECB 加密 CDN 上传）
- 交互式 REPL + 联系人管理
- 守护进程模式：`monitor`（JSON lines）+ `send`（非交互）
- Claude 自动回复机器人，持久化 session + typing 状态 + context 用量追踪
- 语音消息：接收语音转写 + TTS 语音回复（edge-tts）
- Tool 执行进度推送（PostToolUse hook，通知 on/off 切换）
- 微信公众号 webhook：远程登录、会话过期自动重连、消息推送
- Root 管理：服务状态、Bot 控制、模板管理，全在微信里操作
- 多 profile 进程隔离（`WX_PROFILE` 环境变量）

* * *

## 快速开始

```bash
git clone <repo-url> ~/.claude/skills/wx-cli
cd ~/.claude/skills/wx-cli
bash shell/build.sh                        # 编译 bin/wx
cp -r skills/wx-push ~/.claude/skills/wx-push       # 安装推送 skill
cp -r skills/wx-schedule ~/.claude/skills/wx-schedule  # 安装定时任务 skill
bin/wx --profile mybot login               # 扫码登录
bash shell/bot.sh start mybot              # 启动 Claude 自动回复
bash shell/setup-voice.sh                  # 安装语音依赖 (可选, 无需 sudo)
bash tools/setup-hud.sh                    # 配置状态栏 + /usage 用量查询 (可选, 仅订阅模式)
```

* * *

## 命令行

### 登录 & 账号

```bash
bin/wx --profile work login                # 扫码登录，保存为 "work"
bin/wx accounts                            # 列出所有 profile
```

`--profile` 必填。凭证保存到 `~/.wx-cli/accounts/{profile}.json`。

### 交互模式

```bash
bin/wx --profile work
```

| 命令 | 功能 |
|------|------|
| *(文本)* | 回复当前联系人 |
| `/contacts` | 联系人列表 |
| `/use <index>` | 切换联系人 |
| `/name <name>` | 设置昵称 |
| `/image <path>` | 发送图片 |
| `/file <path>` | 发送文件 |
| `/video <path>` | 发送视频 |
| `/status` | 连接信息 |
| `/login` | 重新登录 |
| `/quit` | 退出 |

### 守护进程模式

```bash
# 监听消息 (JSON lines)
bin/wx --profile work monitor

# 非交互发送
bin/wx --profile work send --to USER_ID --ctx CONTEXT_TOKEN --text "hello"
bin/wx --profile work send --to USER_ID --ctx CONTEXT_TOKEN --image /path/to/pic.png
bin/wx --profile work send --to USER_ID --ctx CONTEXT_TOKEN --file /path/to/voice.mp3
```

* * *

## Claude 自动回复

```bash
bash shell/wx-bot.sh work                  # 前台运行
bash shell/bot.sh start work               # 后台托管
bash shell/bot.sh stop work                # 停止
bash shell/bot.sh status work              # 状态
bash shell/bot.sh log work                 # 日志
```

流程: `wx monitor` → JSON 解析 → `shell/handlers/reply.sh` → `claude --print` → `wx send`

**内置命令** (不经过 Claude):

| 命令 | 响应 |
|------|------|
| `/ping` | `pong` |
| `/help` | 命令列表 |
| `/usage` | Context / Usage / Weekly 用量 (bar 格式) |
| `/new` | 清除对话历史，开启新 session |

**行为:**
- 处理文本、语音转写、图片、文件，其他类型返回提示
- Agent 发送图片/文件通过临时 send helper 脚本（封装 ctx，agent 不可见）
- 处理消息时显示"正在输入"typing 状态
- 持久化 session（`--resume`），跨消息保持上下文
- Context 超 80% 自动提醒发 `/new`
- Claude 用量 85% 时自动限流
- CDN 上传失败自动重试 3 次

* * *

## 公众号 Webhook 服务

通过微信公众号测试号实现远程自助登录、断线重连和消息推送，无需登录服务器。

### 启动

```bash
# 通过 ngrok (无公网 IP)
WX_APPID=你的appid WX_SECRET=你的secret WX_ROOT=你的openid bash shell/serve.sh start

# 有公网 IP 或域名 (跳过 ngrok)
WX_APPID=你的appid WX_SECRET=你的secret WX_ROOT=你的openid bash shell/serve.sh start --no-ngrok
```

### 配置测试号

打开 [测试号管理页面](https://mp.weixin.qq.com/debug/cgi-bin/sandboxinfo?action=showinfo&t=sandbox/index)，填写:

- **URL**: `bash shell/serve.sh url` 输出的地址
- **Token**: `cat ~/.wx-cli/serve/wx_token`

> ngrok 免费版每次重启 URL 会变，需重新配置。Token 不变。有公网 IP 的可用 `--no-ngrok` 跳过，直接填 `http://IP:端口`。

### 微信命令

| 命令 | 权限 | 功能 |
|------|------|------|
| `登录` | 所有人 | 获取登录链接，授权后自动启动 bot |
| `状态` | 所有人 | 查看登录状态 |
| `通知 on/off` | 所有人 | Tool 执行进度推送开关 |
| `帮助` | 所有人 | 命令列表 |
| `服务` | root | 服务运行状态 |
| `活跃` | root | 活跃 Bot 列表 |
| `历史` | root | 最新聊天记录 |
| `重启` | root | 重启所有 Bot |
| `关闭` | root | 停止所有 Bot |
| `模板 add/list/del` | root | 推送模板管理 |

每个用户的 OpenID 自动作为 profile，互相隔离。root 通过 `WX_ROOT` 环境变量指定（仅内存，不落盘）。

### 断线自动重连

Bot 检测到会话过期 (code -14) → 通知 serve → 推送新登录链接 → 用户点击授权 → bot 自动重启。

### 消息推送

通过 `wx-push` skill 推送模板消息或纯文本。每个进程持有 server 签发的 session token（PUSH_KEY），绑定单一 profile，无法推给其他用户。

```bash
# bot 进程内 (PUSH_KEY 环境变量自动继承，推给自己)
bash ~/.claude/skills/wx-push/scripts/push.sh --template signal --k1 "AAPL" --k2 "买入" --k3 "策略B" --k4 "14:30"

# crontab (WX_PROFILE 指定身份，push.sh 自动注册临时 token)
WX_PROFILE=oiNG73xxx python3 scan.py AAPL.US --notify push

# root 广播: 通过微信发送 "广播 系统维护通知"
```

### 管理脚本

```bash
bash shell/serve.sh start          # 启动 serve + ngrok
bash shell/serve.sh stop           # 停止
bash shell/serve.sh status         # 运行状态
bash shell/serve.sh url            # 当前公网地址
bash shell/serve.sh log            # 查看日志
bash shell/serve.sh restart-bots   # 重启所有 bot
bash shell/serve.sh stop-bots      # 停止所有 bot
```

* * *

## 项目结构

```
cmd/wx/main.go         入口
internal/
  api/                 协议类型 + HTTP 客户端
  auth/                扫码登录 + 凭证持久化 + 多 profile
  cdn/                 AES-128-ECB 加密 + CDN 上传
  cli/                 交互式 REPL
  msg/                 收发消息 + monitor 循环
  serve/               公众号 webhook 服务
    serve.go           配置 + HTTP 路由
    handler.go         命令分发
    cmd_user.go        登录/状态
    cmd_admin.go       Root 管理命令
    cmd_push.go        推送/注册
    cmd_schedule.go    定时任务命令
    cmd_notify.go      通知开关
    scheduler.go       内置 cron 调度器
    session.go         Push token 管理
    wechat.go          XML 类型、签名、被动/异步/模板回复
    bot.go             Bot 管理 + profile/模板持久化
    state.go           进程状态 + 聊天历史查询
shell/
  build.sh             构建脚本
  bot.sh               Bot 进程管理 (start/stop/status/log)
  serve.sh             Serve + ngrok 管理
  wx-bot.sh            Claude 自动回复守护进程
  setup-voice.sh       语音依赖安装 (edge-tts)
  handlers/reply.sh    消息处理器 (session 管理 + context 追踪)
  handlers/tts.sh      TTS 语音合成 (text → MP3)
  handlers/hook_notify.sh  Tool 执行进度 hook
skills/
  wx-push/             推送 skill (安装到 ~/.claude/skills/wx-push)
  wx-schedule/         定时任务 skill (安装到 ~/.claude/skills/wx-schedule)
tools/
  hud_wrapper.sh       claude-hud 状态栏 + rate_limits 缓存
  setup-hud.sh         一键配置 statusLine
.claude/
  settings.json        PostToolUse hook 配置
prompts/
  system_role.md       系统提示词模板
```

## 数据目录

```
~/.wx-cli/
  accounts/{alias}.json         登录凭证 (alias = sha256(openid)[:12])
  sync/{alias}                  长轮询游标
  notify/{alias}                通知开关标记文件
  pids/                         PID 文件
  logs/                         日志文件
  serve/
    wx_token                    微信验证 Token (自动生成)
    mapping.json                alias → OpenID 映射 (hash → openid)
    templates.json              推送模板注册
    schedules.json              定时任务配置
```

## 协议备注

- API 地址: `https://ilinkai.weixin.qq.com`
- 成功响应: `{}`（无 `ret` 字段）。错误: `{"ret": -2, "errmsg": "..."}`
- 发送封装: `{"msg": {WeixinMessage}}`，`from_user_id` 必须为 `""`
- 媒体 `aes_key`: `base64(utf8_bytes_of_hex_key)` — 不是 `base64(raw_bytes)`
- Token 有效期约 24h，过期需重新登录

## 已知限制

- **语音发送**: iLink Bot API 不支持 bot 发送原生语音条（voice_item type=3），API 返回成功但客户端静默丢弃。TTS 语音以 MP3 文件形式发送。接收语音转写正常。
- **群聊**: iLink API 仅支持 1:1 私聊，不支持群消息。

* * *

## 许可证

MIT

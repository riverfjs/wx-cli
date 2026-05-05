package serve

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

const helpText = "可用命令:\n登录 — 触发 iLink 登录\n状态 — 查看登录状态\n定时 — 管理定时任务\n通知 on/off — Tool 执行进度推送\n帮助 — 显示本帮助"

const rootHelpText = "管理命令 (root):\n服务 — 服务状态\n活跃 — 活跃 Bot 列表\n历史 — 最新聊天记录\n重启 — 重启所有 Bot\n关闭 — 停止所有 Bot\n广播 <消息> — 推送给所有用户\n定时 list — 查看全部定时任务\n模板 add/list/del — 模板管理"

func handleMessage(w http.ResponseWriter, r *http.Request, cfg *Config) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var msg wxMessage
	if err := xml.Unmarshal(body, &msg); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if isDuplicate(msg.MsgId) {
		fmt.Fprint(w, "success")
		return
	}

	if msg.MsgType != "text" {
		replyPassive(w, &msg, helpText)
		return
	}

	content := strings.TrimSpace(msg.Content)
	log.Printf("[wx-serve] from=%s content=%q", msg.FromUserName, content)

	parts := strings.SplitN(content, " ", 2)
	cmd := strings.ToLower(parts[0])
	profile := registerOpenID(msg.FromUserName)
	isRoot := cfg.RootOpenID != "" && msg.FromUserName == cfg.RootOpenID

	switch cmd {
	case "登录", "login":
		handleLogin(w, &msg, cfg, profile)
	case "状态", "status":
		handleStatus(w, &msg, profile)
	case "服务", "service":
		if !isRoot {
			replyPassive(w, &msg, "无权限")
			return
		}
		handleService(w, &msg)
	case "活跃", "bots":
		if !isRoot {
			replyPassive(w, &msg, "无权限")
			return
		}
		handleActiveBots(w, &msg)
	case "历史", "chat":
		if !isRoot {
			replyPassive(w, &msg, "无权限")
			return
		}
		var target string
		if len(parts) >= 2 {
			target = strings.TrimSpace(parts[1])
		}
		handleChatHistory(w, &msg, target)
	case "重启", "restart":
		if !isRoot {
			replyPassive(w, &msg, "无权限")
			return
		}
		handleRestartBots(w, &msg)
	case "关闭", "stopall":
		if !isRoot {
			replyPassive(w, &msg, "无权限")
			return
		}
		handleStopBots(w, &msg)
	case "广播", "broadcast":
		if !isRoot {
			replyPassive(w, &msg, "无权限")
			return
		}
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			replyPassive(w, &msg, "用法: 广播 <消息内容>")
			return
		}
		handleBroadcast(w, &msg, cfg, strings.TrimSpace(parts[1]))
	case "通知", "notify":
		handleNotify(w, &msg, profile, parts)
	case "定时", "cron":
		handleScheduleCmd(w, &msg, cfg, profile, isRoot, content)
	case "模板", "template":
		if !isRoot {
			replyPassive(w, &msg, "无权限")
			return
		}
		handleTemplate(w, &msg, content)
	case "帮助", "help":
		h := helpText
		if isRoot {
			h += "\n\n" + rootHelpText
		}
		replyPassive(w, &msg, h)
	default:
		replyPassive(w, &msg, helpText)
	}
}

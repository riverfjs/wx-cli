package serve

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"wx-cli/internal/api"
	"wx-cli/internal/auth"
)

type loginSession struct {
	profile   string
	wxUser    string
	startTime time.Time
}

var (
	sessions   = make(map[string]*loginSession)
	sessionsMu sync.Mutex
)

const helpText = "可用命令:\n登录 — 触发 iLink 登录\n状态 — 查看登录状态\n帮助 — 显示本帮助"

const rootHelpText = "管理命令 (root):\n服务 — 服务状态\n活跃 — 活跃 Bot 列表\n历史 — 最新聊天记录\n重启 — 重启所有 Bot\n关闭 — 停止所有 Bot\n广播 <消息> — 推送给所有用户\n模板 add/list/del — 模板管理"

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
		text := strings.TrimSpace(parts[1])
		aliases := listManagedAliases()
		count := 0
		for _, a := range aliases {
			if oid, ok := resolveAlias(a); ok {
				sendAsync(cfg, oid, text)
				count++
			}
		}
		replyPassive(w, &msg, fmt.Sprintf("已广播给 %d 个用户", count))
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

func handleLogin(w http.ResponseWriter, msg *wxMessage, cfg *Config, profile string) {
	sessionsMu.Lock()
	if s, ok := sessions[profile]; ok {
		sessionsMu.Unlock()
		ago := time.Since(s.startTime).Truncate(time.Second)
		replyPassive(w, msg, fmt.Sprintf("正在登录中(%s前发起)，请稍候...", ago))
		return
	}

	qrcode, qrURL, err := api.GetQrCode()
	if err != nil {
		sessionsMu.Unlock()
		replyPassive(w, msg, fmt.Sprintf("获取二维码失败: %s", err))
		return
	}

	sess := &loginSession{
		profile:   profile,
		wxUser:    msg.FromUserName,
		startTime: time.Now(),
	}
	sessions[profile] = sess
	sessionsMu.Unlock()

	replyPassive(w, msg, fmt.Sprintf("请点击链接扫码登录:\n\n%s\n\n二维码2分钟内有效", qrURL))

	go pollLogin(cfg, sess, qrcode)
}

func handleRelogin(w http.ResponseWriter, r *http.Request, cfg *Config) {
	profile := r.FormValue("profile")
	if profile == "" {
		http.Error(w, "missing profile", http.StatusBadRequest)
		return
	}

	openID, ok := resolveAlias(profile)
	if !ok {
		log.Printf("[wx-serve] relogin ignored, unknown profile: %s", profile)
		http.Error(w, "unknown profile", http.StatusForbidden)
		return
	}

	log.Printf("[wx-serve] relogin triggered for profile %s", profile)

	sendAsync(cfg, openID, "Bot 会话已过期，正在生成登录链接...")

	sessionsMu.Lock()
	if s, ok := sessions[profile]; ok {
		sessionsMu.Unlock()
		ago := time.Since(s.startTime).Truncate(time.Second)
		log.Printf("[wx-serve] relogin skipped, already in progress (%s ago)", ago)
		fmt.Fprint(w, "already in progress")
		return
	}

	qrcode, qrURL, err := api.GetQrCode()
	if err != nil {
		sessionsMu.Unlock()
		log.Printf("[wx-serve] relogin QR failed: %s", err)
		sendAsync(cfg, openID, "重新登录失败: "+err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sess := &loginSession{
		profile:   profile,
		wxUser:    openID,
		startTime: time.Now(),
	}
	sessions[profile] = sess
	sessionsMu.Unlock()

	sendAsync(cfg, openID, fmt.Sprintf("请点击链接重新登录:\n\n%s\n\n二维码2分钟内有效", qrURL))

	go pollLogin(cfg, sess, qrcode)

	fmt.Fprint(w, "relogin initiated")
}

func handleStatus(w http.ResponseWriter, msg *wxMessage, profile string) {
	cred := auth.LoadCredential(profile)
	if cred == nil {
		replyPassive(w, msg, "未找到登录信息")
		return
	}
	loginTime := cred.LoginTime
	if t, err := time.Parse(time.RFC3339, cred.LoginTime); err == nil {
		loginTime = t.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05")
	}
	replyPassive(w, msg, fmt.Sprintf("Bot ID: %s\n登录时间: %s", cred.BotID, loginTime))
}

func handleService(w http.ResponseWriter, msg *wxMessage) {
	fmtProc := func(p ProcessInfo) string {
		if !p.Alive {
			return p.Name + ": 未运行"
		}
		return fmt.Sprintf("%s: 运行中 (PID %d)", p.Name, p.PID)
	}

	lines := []string{
		fmtProc(checkProcess("serve")),
		fmtProc(checkProcess("ngrok")),
		fmt.Sprintf("管理 profile 数: %d", len(listManagedAliases())),
		fmt.Sprintf("注册模板数: %d", len(listTemplates())),
	}
	replyPassive(w, msg, strings.Join(lines, "\n"))
}

func handleActiveBots(w http.ResponseWriter, msg *wxMessage) {
	bots := listActiveBots()
	if len(bots) == 0 {
		replyPassive(w, msg, "无活跃 Bot")
		return
	}
	var lines []string
	for _, b := range bots {
		lines = append(lines, fmt.Sprintf("%s (PID %d)", shortID(b.Profile), b.PID))
	}
	replyPassive(w, msg, fmt.Sprintf("活跃 Bot (%d):\n%s", len(lines), strings.Join(lines, "\n")))
}

func handleChatHistory(w http.ResponseWriter, msg *wxMessage, target string) {
	var profiles []string
	if target != "" {
		profiles = []string{target}
	} else {
		for _, b := range listActiveBots() {
			profiles = append(profiles, b.Profile)
		}
	}
	if len(profiles) == 0 {
		replyPassive(w, msg, "无活跃 Bot 或未指定 profile")
		return
	}

	var lines []string
	for _, profile := range profiles {
		chat := latestChat(profile)
		label := shortID(profile)
		if chat == nil {
			lines = append(lines, fmt.Sprintf("[%s] 无聊天记录", label))
			continue
		}
		ago := time.Since(chat.ModTime).Truncate(time.Minute)
		lines = append(lines, fmt.Sprintf("[%s] %s (%s前)\nQ: %s\nA: %s",
			label, shortID(chat.UserID), ago,
			truncate(chat.Question, 50), truncate(chat.Answer, 80)))
	}
	replyPassive(w, msg, strings.Join(lines, "\n\n"))
}

func handleRestartBots(w http.ResponseWriter, msg *wxMessage) {
	bots := listActiveBots()
	if len(bots) == 0 {
		replyPassive(w, msg, "无活跃 Bot")
		return
	}
	var lines []string
	for _, b := range bots {
		if err := restartBot(b.Profile); err != nil {
			lines = append(lines, fmt.Sprintf("%s: 重启失败 %s", shortID(b.Profile), err))
		} else {
			lines = append(lines, fmt.Sprintf("%s: 已重启", shortID(b.Profile)))
		}
	}
	replyPassive(w, msg, fmt.Sprintf("重启 %d 个 Bot:\n%s", len(bots), strings.Join(lines, "\n")))
}

func handleStopBots(w http.ResponseWriter, msg *wxMessage) {
	bots := listActiveBots()
	if len(bots) == 0 {
		replyPassive(w, msg, "无活跃 Bot")
		return
	}
	var lines []string
	for _, b := range bots {
		if err := stopBot(b.Profile); err != nil {
			lines = append(lines, fmt.Sprintf("%s: 停止失败 %s", shortID(b.Profile), err))
		} else {
			lines = append(lines, fmt.Sprintf("%s: 已停止", shortID(b.Profile)))
		}
	}
	replyPassive(w, msg, fmt.Sprintf("停止 %d 个 Bot:\n%s", len(bots), strings.Join(lines, "\n")))
}

func handleTemplate(w http.ResponseWriter, msg *wxMessage, content string) {
	parts := strings.Fields(content)
	if len(parts) < 2 {
		replyPassive(w, msg, "用法:\n模板 add <name> <id>\n模板 list\n模板 del <name>")
		return
	}

	action := strings.ToLower(parts[1])
	switch action {
	case "add":
		if len(parts) < 4 {
			replyPassive(w, msg, "用法: 模板 add <name> <template_id>")
			return
		}
		setTemplate(parts[2], parts[3])
		replyPassive(w, msg, fmt.Sprintf("模板已注册: %s", parts[2]))
	case "list":
		tpls := listTemplates()
		if len(tpls) == 0 {
			replyPassive(w, msg, "无已注册模板")
			return
		}
		var lines []string
		for name, id := range tpls {
			lines = append(lines, fmt.Sprintf("%s: %s", name, id[:8]+"..."))
		}
		replyPassive(w, msg, strings.Join(lines, "\n"))
	case "del":
		if len(parts) < 3 {
			replyPassive(w, msg, "用法: 模板 del <name>")
			return
		}
		if delTemplate(parts[2]) {
			replyPassive(w, msg, fmt.Sprintf("模板已删除: %s", parts[2]))
		} else {
			replyPassive(w, msg, fmt.Sprintf("模板不存在: %s", parts[2]))
		}
	default:
		replyPassive(w, msg, "用法:\n模板 add <name> <id>\n模板 list\n模板 del <name>")
	}
}

func handleRegister(w http.ResponseWriter, r *http.Request, cfg *Config) {
	profile := r.FormValue("profile")
	secret := r.FormValue("secret")
	ttlParam := r.FormValue("ttl")

	if profile == "" || secret == "" {
		http.Error(w, "missing profile or secret", http.StatusBadRequest)
		return
	}
	if secret != cfg.WxToken {
		http.Error(w, "invalid secret", http.StatusForbidden)
		return
	}

	var ttl time.Duration
	if ttlParam != "" {
		if d, err := time.ParseDuration(ttlParam); err == nil {
			ttl = d
		}
	}

	token := registerPushToken(profile, ttl)
	label := "permanent"
	if ttl > 0 {
		label = ttl.String()
	}
	log.Printf("[wx-serve] registered push token for %s (ttl=%s)", shortID(profile), label)
	fmt.Fprint(w, token)
}

func handlePush(w http.ResponseWriter, r *http.Request, cfg *Config) {
	r.ParseForm()
	token := r.FormValue("token")
	text := r.FormValue("text")
	tplName := r.FormValue("template")

	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	alias, ok := lookupPushToken(token)
	if !ok {
		http.Error(w, "invalid or expired token", http.StatusForbidden)
		return
	}

	openID, ok := resolveAlias(alias)
	if !ok {
		http.Error(w, "profile mapping not found", http.StatusInternalServerError)
		return
	}

	var keywords []string
	var templateID string
	if tplName != "" {
		templateID = getTemplate(tplName)
		if templateID == "" {
			http.Error(w, "template not found: "+tplName, http.StatusBadRequest)
			return
		}
		for i := 1; ; i++ {
			v := r.FormValue(fmt.Sprintf("k%d", i))
			if v == "" {
				break
			}
			keywords = append(keywords, v)
		}
		if len(keywords) == 0 {
			http.Error(w, "missing keyword params (k1, k2, ...)", http.StatusBadRequest)
			return
		}
	} else if text == "" {
		http.Error(w, "missing template or text", http.StatusBadRequest)
		return
	}

	if templateID != "" {
		if err := sendTemplate(cfg, openID, templateID, keywords); err != nil {
			log.Printf("[wx-serve] push failed for %s: %s", alias, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		sendAsync(cfg, openID, text)
	}

	log.Printf("[wx-serve] push to=%s tpl=%s", alias, tplName)
	fmt.Fprint(w, "ok")
}

func pollLogin(cfg *Config, sess *loginSession, qrcode string) {
	defer func() {
		sessionsMu.Lock()
		delete(sessions, sess.profile)
		sessionsMu.Unlock()
	}()

	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			log.Printf("[wx-serve] login timeout for profile %s", sess.profile)
			sendAsync(cfg, sess.wxUser, "登录超时，二维码已过期。请重新发送: 登录")
			return
		case <-ticker.C:
			status, cred, err := api.GetQrStatus(qrcode)
			if err != nil {
				continue
			}
			switch status {
			case "expired":
				log.Printf("[wx-serve] QR expired for profile %s", sess.profile)
				sendAsync(cfg, sess.wxUser, "二维码已过期。请重新发送: 登录")
				return
			case "confirmed":
				if cred == nil {
					continue
				}
				path := auth.SaveCredential(cred, sess.profile)
				log.Printf("[wx-serve] login success for profile %s, saved to %s", sess.profile, path)
				msg := fmt.Sprintf("登录成功!\nBot ID: %s", cred.BotID)
				if err := startBot(sess.profile); err != nil {
					log.Printf("[wx-serve] bot start failed for %s: %s", sess.profile, err)
					msg += "\n\nBot 启动失败: " + err.Error()
				} else {
					log.Printf("[wx-serve] bot started for profile %s", sess.profile)
					msg += "\n\nBot 已自动启动"
				}
				sendAsync(cfg, sess.wxUser, msg)
				return
			}
		}
	}
}

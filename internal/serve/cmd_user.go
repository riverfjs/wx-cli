package serve

import (
	"fmt"
	"log"
	"net/http"
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

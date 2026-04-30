package serve

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type wxMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgId        int64    `xml:"MsgId"`
}

type wxReply struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
}

var (
	recentMsgs   = make(map[int64]time.Time)
	recentMsgsMu sync.Mutex
)

var (
	cachedToken    string
	tokenExpiresAt time.Time
	tokenMu        sync.Mutex
)

func handleVerify(w http.ResponseWriter, r *http.Request, token string) {
	q := r.URL.Query()
	signature := q.Get("signature")
	timestamp := q.Get("timestamp")
	nonce := q.Get("nonce")
	echostr := q.Get("echostr")

	strs := []string{token, timestamp, nonce}
	sort.Strings(strs)
	h := sha1.New()
	h.Write([]byte(strings.Join(strs, "")))
	computed := fmt.Sprintf("%x", h.Sum(nil))

	if computed == signature {
		fmt.Fprint(w, echostr)
		log.Println("[wx-serve] token verification OK")
	} else {
		w.WriteHeader(http.StatusForbidden)
		log.Println("[wx-serve] token verification failed")
	}
}

func isDuplicate(msgId int64) bool {
	recentMsgsMu.Lock()
	defer recentMsgsMu.Unlock()
	if _, seen := recentMsgs[msgId]; seen {
		return true
	}
	recentMsgs[msgId] = time.Now()
	return false
}

func replyPassive(w http.ResponseWriter, msg *wxMessage, content string) {
	reply := wxReply{
		ToUserName:   msg.FromUserName,
		FromUserName: msg.ToUserName,
		CreateTime:   time.Now().Unix(),
		MsgType:      "text",
		Content:      content,
	}
	data, _ := xml.Marshal(&reply)
	w.Header().Set("Content-Type", "application/xml")
	w.Write(data)
}

func sendAsync(cfg *Config, toUser, content string) {
	if cfg.WxAppID == "" || cfg.WxSecret == "" {
		log.Printf("[wx-serve] async send skipped (no appid/secret): %s", content)
		return
	}

	token, err := getAccessToken(cfg)
	if err != nil {
		log.Printf("[wx-serve] get access_token failed: %s", err)
		return
	}

	payload := map[string]interface{}{
		"touser":  toUser,
		"msgtype": "text",
		"text":    map[string]string{"content": content},
	}
	data, _ := json.Marshal(payload)

	resp, err := http.Post(
		"https://api.weixin.qq.com/cgi-bin/message/custom/send?access_token="+token,
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		log.Printf("[wx-serve] async send failed: %s", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &result)
	if result.ErrCode != 0 {
		log.Printf("[wx-serve] async send error: %d %s", result.ErrCode, result.ErrMsg)
	}
}

func sendTemplate(cfg *Config, toUser, templateID string, keywords []string) error {
	if cfg.WxAppID == "" || cfg.WxSecret == "" {
		return fmt.Errorf("appid/secret not configured")
	}

	token, err := getAccessToken(cfg)
	if err != nil {
		return err
	}

	data_fields := make(map[string]interface{})
	for i, v := range keywords {
		data_fields[fmt.Sprintf("keyword%d", i+1)] = map[string]string{"value": v}
	}

	payload := map[string]interface{}{
		"touser":      toUser,
		"template_id": templateID,
		"data":        data_fields,
	}
	data, _ := json.Marshal(payload)

	resp, err := http.Post(
		"https://api.weixin.qq.com/cgi-bin/message/template/send?access_token="+token,
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &result)
	if result.ErrCode != 0 {
		return fmt.Errorf("%d %s", result.ErrCode, result.ErrMsg)
	}
	return nil
}

func getAccessToken(cfg *Config) (string, error) {
	tokenMu.Lock()
	defer tokenMu.Unlock()

	if cachedToken != "" && time.Now().Before(tokenExpiresAt) {
		return cachedToken, nil
	}

	payload := map[string]string{
		"grant_type": "client_credential",
		"appid":      cfg.WxAppID,
		"secret":     cfg.WxSecret,
	}
	data, _ := json.Marshal(payload)

	resp, err := http.Post(
		"https://api.weixin.qq.com/cgi-bin/stable_token",
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &result)

	if result.AccessToken == "" {
		return "", fmt.Errorf("get access_token: %d %s", result.ErrCode, result.ErrMsg)
	}

	cachedToken = result.AccessToken
	tokenExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)
	return cachedToken, nil
}

func cleanupRecentMsgs() {
	for {
		time.Sleep(30 * time.Second)
		recentMsgsMu.Lock()
		cutoff := time.Now().Add(-30 * time.Second)
		for id, t := range recentMsgs {
			if t.Before(cutoff) {
				delete(recentMsgs, id)
			}
		}
		recentMsgsMu.Unlock()
	}
}

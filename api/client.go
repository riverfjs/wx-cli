package api

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	LoginBase      = "https://ilinkai.weixin.qq.com"
	channelVersion = "1.0.11"
)

var Client = &http.Client{Timeout: 45 * time.Second}

func newBaseInfo() BaseInfo {
	return BaseInfo{ChannelVersion: channelVersion}
}

func randomUin() string {
	b := make([]byte, 4)
	rand.Read(b)
	v := binary.BigEndian.Uint32(b)
	return base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(uint64(v), 10)))
}

func headers(token string) map[string]string {
	h := map[string]string{
		"Content-Type":      "application/json",
		"X-WECHAT-UIN":     randomUin(),
		"iLink-App-Id":     "bot",
		"AuthorizationType": "ilink_bot_token",
	}
	if token != "" {
		h["Authorization"] = "Bearer " + token
	}
	return h
}

func Post(baseURL, endpoint string, body interface{}, token string) ([]byte, error) {
	url := strings.TrimRight(baseURL, "/") + "/" + endpoint
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
	for k, v := range headers(token) {
		req.Header.Set(k, v)
	}
	resp, err := Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func Get(url string) ([]byte, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("iLink-App-Id", "bot")
	resp, err := Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// ── iLink API endpoints ──

func GetQrCode() (qrcode, qrURL string, err error) {
	data, err := Get(LoginBase + "/ilink/bot/get_bot_qrcode?bot_type=3")
	if err != nil {
		return "", "", err
	}
	var resp struct {
		Ret   int    `json:"ret"`
		QR    string `json:"qrcode"`
		QRImg string `json:"qrcode_img_content"`
	}
	json.Unmarshal(data, &resp)
	if resp.Ret != 0 {
		return "", "", fmt.Errorf("get_bot_qrcode ret=%d", resp.Ret)
	}
	return resp.QR, resp.QRImg, nil
}

func GetQrStatus(qrcode string) (status string, cred *Credential, err error) {
	data, err := Get(LoginBase + "/ilink/bot/get_qrcode_status?qrcode=" + qrcode)
	if err != nil {
		return "", nil, err
	}
	var resp struct {
		Status  string `json:"status"`
		Token   string `json:"bot_token"`
		BaseURL string `json:"baseurl"`
		BotID   string `json:"ilink_bot_id"`
		UserID  string `json:"ilink_user_id"`
	}
	json.Unmarshal(data, &resp)
	if resp.Status == "confirmed" {
		base := resp.BaseURL
		if base == "" {
			base = LoginBase
		}
		cred = &Credential{
			BotToken:  resp.Token,
			BaseURL:   base,
			BotID:     resp.BotID,
			UserID:    resp.UserID,
			LoginTime: time.Now().UTC().Format(time.RFC3339),
		}
	}
	return resp.Status, cred, nil
}

func GetUpdates(cred *Credential, syncBuf string) (*GetUpdatesResp, error) {
	body := map[string]interface{}{
		"get_updates_buf": syncBuf,
		"base_info":       newBaseInfo(),
	}
	data, err := Post(cred.BaseURL, "ilink/bot/getupdates", body, cred.BotToken)
	if err != nil {
		return nil, err
	}
	var resp GetUpdatesResp
	json.Unmarshal(data, &resp)
	return &resp, nil
}

func SendMessage(cred *Credential, msg *WeixinMessage) error {
	body := map[string]interface{}{
		"msg":       msg,
		"base_info": newBaseInfo(),
	}
	data, err := Post(cred.BaseURL, "ilink/bot/sendmessage", body, cred.BotToken)
	if err != nil {
		return err
	}
	var resp struct {
		Ret *int `json:"ret,omitempty"`
	}
	json.Unmarshal(data, &resp)
	if resp.Ret != nil && *resp.Ret != 0 {
		return fmt.Errorf("sendmessage ret=%d: %s", *resp.Ret, string(data))
	}
	return nil
}

func GetUploadURL(cred *Credential, params map[string]interface{}) (*UploadURLResp, error) {
	params["base_info"] = newBaseInfo()
	data, err := Post(cred.BaseURL, "ilink/bot/getuploadurl", params, cred.BotToken)
	if err != nil {
		return nil, err
	}
	var resp UploadURLResp
	json.Unmarshal(data, &resp)
	if resp.Ret != nil && *resp.Ret != 0 {
		return nil, fmt.Errorf("getuploadurl ret=%d", *resp.Ret)
	}
	return &resp, nil
}

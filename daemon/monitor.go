package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"wx-cli/api"
	"wx-cli/auth"
	"wx-cli/cdn"
)

type MessageEvent struct {
	FromUserID   string `json:"from_user_id"`
	ContextToken string `json:"context_token"`
	Type         string `json:"type"` // text, image, voice, file, video
	Text         string `json:"text,omitempty"`
	FileName     string `json:"file_name,omitempty"`
	ImagePath    string `json:"image_path,omitempty"`
	FilePath     string `json:"file_path,omitempty"`
	Time         string `json:"time"`
}

func downloadImage(media *api.CDNMedia, from string) string {
	if media == nil || media.EncryptQueryParam == "" {
		return ""
	}
	data, err := cdn.Download(media)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[wx-monitor] image download: %s\n", err)
		return ""
	}
	dir := filepath.Join(os.TempDir(), "wx-images")
	os.MkdirAll(dir, 0755)
	name := fmt.Sprintf("%s_%d.jpg", from[:min(8, len(from))], time.Now().UnixMilli())
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "[wx-monitor] image save: %s\n", err)
		return ""
	}
	return path
}

func downloadFile(media *api.CDNMedia, fileName string) string {
	if media == nil || media.EncryptQueryParam == "" {
		return ""
	}
	data, err := cdn.Download(media)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[wx-monitor] file download: %s\n", err)
		return ""
	}
	dir := filepath.Join(os.TempDir(), "wx-files")
	os.MkdirAll(dir, 0755)
	if fileName == "" {
		fileName = fmt.Sprintf("file_%d", time.Now().UnixMilli())
	}
	path := filepath.Join(dir, fileName)
	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "[wx-monitor] file save: %s\n", err)
		return ""
	}
	return path
}

func profileHint(profile string) string {
	if profile != "" {
		return " --profile " + profile
	}
	return ""
}

func Monitor(cred *api.Credential, profile string) {
	syncBuf := auth.LoadSyncBuf(profile)
	errCount := 0

	fmt.Fprintln(os.Stderr, "[wx-monitor] started, waiting for messages...")

	for {
		resp, err := api.GetUpdates(cred, syncBuf)
		if err != nil {
			errCount++
			if errCount > 20 {
				fmt.Fprintln(os.Stderr, "[wx-monitor] too many errors, exiting")
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "[wx-monitor] error: %s\n", err)
			time.Sleep(3 * time.Second)
			continue
		}

		code := 0
		if resp.Ret != nil {
			code = *resp.Ret
		} else if resp.ErrCode != nil {
			code = *resp.ErrCode
		}

		if code == -14 {
			fmt.Fprintln(os.Stderr, "[wx-monitor] session expired, please re-login: wx login"+profileHint(profile))
			os.Exit(1)
		}
		if code != 0 {
			fmt.Fprintf(os.Stderr, "[wx-monitor] ret=%d %s\n", code, resp.ErrMsg)
			errCount++
			time.Sleep(3 * time.Second)
			continue
		}

		errCount = 0
		if resp.GetUpdatesBuf != "" {
			syncBuf = resp.GetUpdatesBuf
			auth.SaveSyncBuf(profile, syncBuf)
		}

		for _, msg := range resp.Msgs {
			if msg.MessageType == 2 {
				continue // skip bot's own messages
			}
			for _, item := range msg.ItemList {
				ev := MessageEvent{
					FromUserID:   msg.FromUserID,
					ContextToken: msg.ContextToken,
					Time:         time.Now().Format("15:04:05"),
				}
				switch item.Type {
				case 1:
					ev.Type = "text"
					if item.TextItem != nil {
						ev.Text = item.TextItem.Text
					}
				case 2:
					ev.Type = "image"
					if item.ImageItem != nil {
						ev.ImagePath = downloadImage(item.ImageItem.Media, msg.FromUserID)
					}
				case 3:
					ev.Type = "voice"
					if item.VoiceItem != nil && item.VoiceItem.Text != "" {
						ev.Text = item.VoiceItem.Text
					}
				case 4:
					ev.Type = "file"
					if item.FileItem != nil {
						ev.FileName = item.FileItem.FileName
						ev.FilePath = downloadFile(item.FileItem.Media, item.FileItem.FileName)
					}
				case 5:
					ev.Type = "video"
				default:
					continue
				}
				data, _ := json.Marshal(ev)
				fmt.Println(string(data))
			}
		}
	}
}

package msg

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"wx-cli/internal/api"
	"wx-cli/internal/auth"
)

func profileHint(profile string) string {
	if profile != "" {
		return " --profile " + profile
	}
	return ""
}

// Monitor runs the long-poll loop, emitting one JSON line per message event.
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

			// Persist context token for this user
			if msg.ContextToken != "" && msg.FromUserID != "" {
				auth.SaveContextToken(profile, msg.FromUserID, msg.ContextToken)
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

// Start runs a long-poll loop calling a handler for each message, used by the interactive REPL.
type Handler func(msg *api.WeixinMessage)

func Start(cred *api.Credential, profile string, handler Handler, done <-chan struct{}) {
	syncBuf := auth.LoadSyncBuf(profile)
	errCount := 0

	for {
		select {
		case <-done:
			return
		default:
		}

		resp, err := api.GetUpdates(cred, syncBuf)
		if err != nil {
			errCount++
			if errCount > 10 {
				fmt.Fprintln(os.Stderr, "[monitor] Too many errors, stopping.")
				return
			}
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
			time.Sleep(5 * time.Second)
			continue
		}
		if code != 0 {
			fmt.Fprintf(os.Stderr, "[monitor] Error: ret=%d %s\n", code, resp.ErrMsg)
			errCount++
			time.Sleep(3 * time.Second)
			continue
		}

		errCount = 0
		if resp.GetUpdatesBuf != "" {
			syncBuf = resp.GetUpdatesBuf
			auth.SaveSyncBuf(profile, syncBuf)
		}
		for _, m := range resp.Msgs {
			// Persist context token for this user
			if m.ContextToken != "" && m.FromUserID != "" {
				auth.SaveContextToken(profile, m.FromUserID, m.ContextToken)
			}
			handler(m)
		}
	}
}

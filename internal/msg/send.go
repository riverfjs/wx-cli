package msg

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"wx-cli/internal/api"
	"wx-cli/internal/cdn"
)

func clientID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("wx-cli-%d-%s", time.Now().UnixMilli(), hex.EncodeToString(b))
}

func buildMsg(to, ctx string, item *api.MessageItem) *api.WeixinMessage {
	return &api.WeixinMessage{
		FromUserID:   "",
		ToUserID:     to,
		ClientID:     clientID(),
		MessageType:  2,
		MessageState: 2,
		ItemList:     []*api.MessageItem{item},
		ContextToken: ctx,
	}
}

func Text(cred *api.Credential, to, ctx, text string) error {
	return api.SendMessage(cred, buildMsg(to, ctx, &api.MessageItem{
		Type:     1,
		TextItem: &api.TextItem{Text: text},
	}))
}

func Image(cred *api.Credential, to, ctx, filePath string) error {
	up, err := cdn.Upload(cred, filePath, to)
	if err != nil {
		return err
	}
	return api.SendMessage(cred, buildMsg(to, ctx, &api.MessageItem{
		Type: 2,
		ImageItem: &api.ImageItem{
			Media:   cdn.BuildCDNMedia(up),
			MidSize: up.FileSizeCiphertext,
		},
	}))
}

func File(cred *api.Credential, to, ctx, filePath string) error {
	up, err := cdn.Upload(cred, filePath, to)
	if err != nil {
		return err
	}
	return api.SendMessage(cred, buildMsg(to, ctx, &api.MessageItem{
		Type: 4,
		FileItem: &api.FileItem{
			Media:    cdn.BuildCDNMedia(up),
			FileName: up.FileName,
			Len:      strconv.Itoa(up.FileSize),
		},
	}))
}

func Voice(cred *api.Credential, to, ctx, filePath string, durationMs int) error {
	up, err := cdn.Upload(cred, filePath, to)
	if err != nil {
		return err
	}
	return api.SendMessage(cred, buildMsg(to, ctx, &api.MessageItem{
		Type: 3,
		VoiceItem: &api.VoiceItem{
			Media:         cdn.BuildCDNMedia(up),
			EncodeType:    6,
			BitsPerSample: 16,
			SampleRate:    24000,
			Playtime:      durationMs,
		},
	}))
}

func Video(cred *api.Credential, to, ctx, filePath string) error {
	up, err := cdn.Upload(cred, filePath, to)
	if err != nil {
		return err
	}
	return api.SendMessage(cred, buildMsg(to, ctx, &api.MessageItem{
		Type: 5,
		VideoItem: &api.VideoItem{
			Media:     cdn.BuildCDNMedia(up),
			VideoSize: up.FileSizeCiphertext,
		},
	}))
}

package msg

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"wx-cli/internal/api"
	"wx-cli/internal/cdn"
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

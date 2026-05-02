package cdn

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"wx-cli/internal/api"
)

const cdnBase = "https://novac2c.cdn.weixin.qq.com/c2c"

type UploadedFile struct {
	DownloadParam      string
	AESKeyHex          string
	FileSize           int
	FileSizeCiphertext int
	FileName           string
	MediaType          int // 1=image 2=video 3=file
}

func DetectMediaType(path string) int {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return 1
	case ".mp4", ".avi", ".mov", ".mkv":
		return 2
	case ".silk", ".slk":
		return 3
	default:
		return 3
	}
}

func Upload(cred *api.Credential, filePath, toUserID string) (*UploadedFile, error) {
	plaintext, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	h := md5.Sum(plaintext)
	rawMD5 := hex.EncodeToString(h[:])
	aesKey := make([]byte, 16)
	rand.Read(aesKey)
	aesKeyHex := hex.EncodeToString(aesKey)
	fk := make([]byte, 16)
	rand.Read(fk)
	filekey := hex.EncodeToString(fk)
	ciphertext := AesEcbEncrypt(plaintext, aesKey)
	mt := DetectMediaType(filePath)

	urlResp, err := api.GetUploadURL(cred, map[string]interface{}{
		"filekey":       filekey,
		"media_type":    mt,
		"to_user_id":    toUserID,
		"rawsize":       len(plaintext),
		"rawfilemd5":    rawMD5,
		"filesize":      len(ciphertext),
		"no_need_thumb": true,
		"aeskey":        aesKeyHex,
	})
	if err != nil {
		return nil, err
	}

	cdnURL := urlResp.UploadFullURL
	if cdnURL == "" && urlResp.UploadParam != "" {
		cdnURL = cdnBase + "/upload?encrypted_query_param=" +
			url.QueryEscape(urlResp.UploadParam) + "&filekey=" + url.QueryEscape(filekey)
	}
	if cdnURL == "" {
		return nil, fmt.Errorf("no upload URL returned")
	}

	req, _ := http.NewRequest("POST", cdnURL, bytes.NewReader(ciphertext))
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := api.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	dlParam := resp.Header.Get("x-encrypted-param")
	if dlParam == "" {
		return nil, fmt.Errorf("CDN missing x-encrypted-param header")
	}

	return &UploadedFile{
		DownloadParam:      dlParam,
		AESKeyHex:          aesKeyHex,
		FileSize:           len(plaintext),
		FileSizeCiphertext: AesEcbPaddedSize(len(plaintext)),
		FileName:           filepath.Base(filePath),
		MediaType:          mt,
	}, nil
}

// BuildMediaAESKey: base64(utf8_bytes_of_hex_string) per vendor/send.ts:170
func BuildMediaAESKey(aesKeyHex string) string {
	return base64.StdEncoding.EncodeToString([]byte(aesKeyHex))
}

func BuildCDNMedia(up *UploadedFile) *api.CDNMedia {
	return &api.CDNMedia{
		EncryptQueryParam: up.DownloadParam,
		AESKey:            BuildMediaAESKey(up.AESKeyHex),
		EncryptType:       1,
	}
}

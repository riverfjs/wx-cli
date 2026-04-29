package cdn

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"wx-cli/internal/api"
)

// Download fetches and decrypts a CDN media file.
// aesKeyB64 is base64(utf8_bytes_of_hex_string) as stored in the protocol.
func Download(media *api.CDNMedia) ([]byte, error) {
	if media == nil || media.EncryptQueryParam == "" || media.AESKey == "" {
		return nil, fmt.Errorf("missing media or aes_key")
	}

	// Decode aes_key: base64 → hex string → raw bytes
	hexBytes, err := base64.StdEncoding.DecodeString(media.AESKey)
	if err != nil {
		return nil, fmt.Errorf("decode aes_key base64: %w", err)
	}
	key, err := hex.DecodeString(string(hexBytes))
	if err != nil {
		return nil, fmt.Errorf("decode aes_key hex: %w", err)
	}

	dlURL := cdnBase + "/download?encrypted_query_param=" + url.QueryEscape(media.EncryptQueryParam)
	resp, err := http.Get(dlURL)
	if err != nil {
		return nil, fmt.Errorf("cdn download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("cdn download status %d", resp.StatusCode)
	}

	ciphertext, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cdn read body: %w", err)
	}

	return AesEcbDecrypt(ciphertext, key)
}

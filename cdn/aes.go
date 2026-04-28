package cdn

import (
	"bytes"
	"crypto/aes"
)

func AesEcbEncrypt(plaintext, key []byte) []byte {
	block, _ := aes.NewCipher(key)
	bs := block.BlockSize()
	padding := bs - len(plaintext)%bs
	plaintext = append(plaintext, bytes.Repeat([]byte{byte(padding)}, padding)...)
	out := make([]byte, len(plaintext))
	for i := 0; i < len(plaintext); i += bs {
		block.Encrypt(out[i:i+bs], plaintext[i:i+bs])
	}
	return out
}

func AesEcbPaddedSize(n int) int {
	return ((n + 1 + 15) / 16) * 16
}

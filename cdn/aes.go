package cdn

import (
	"bytes"
	"crypto/aes"
	"fmt"
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

func AesEcbDecrypt(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	bs := block.BlockSize()
	if len(ciphertext)%bs != 0 {
		return nil, fmt.Errorf("ciphertext length %d not multiple of block size %d", len(ciphertext), bs)
	}
	out := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i += bs {
		block.Decrypt(out[i:i+bs], ciphertext[i:i+bs])
	}
	// PKCS7 unpad
	if len(out) == 0 {
		return out, nil
	}
	pad := int(out[len(out)-1])
	if pad > bs || pad == 0 {
		return out, nil
	}
	return out[:len(out)-pad], nil
}

func AesEcbPaddedSize(n int) int {
	return ((n + 1 + 15) / 16) * 16
}

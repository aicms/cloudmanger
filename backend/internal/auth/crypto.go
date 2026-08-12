package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// AESEncryptor 使用 AES-GCM 加密云账号密钥
type AESEncryptor struct {
	key []byte
}

func NewAESEncryptor(key string) *AESEncryptor {
	k := []byte(key)
	// 保证 32 字节
	if len(k) > 32 {
		k = k[:32]
	} else if len(k) < 32 {
		pad := make([]byte, 32-len(k))
		k = append(k, pad...)
	}
	return &AESEncryptor{key: k}
}

// Encrypt 加密返回 base64 字符串
func (e *AESEncryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密
func (e *AESEncryptor) Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce := ciphertext[:gcm.NonceSize()]
	plain, err := gcm.Open(nil, nonce, ciphertext[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

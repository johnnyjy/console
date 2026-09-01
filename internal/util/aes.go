package util

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
)

// AesEncrypt 对应 AesUtil.encrypt（AES/CBC/PKCS5Padding，Base64 输出）
func AesEncrypt(key, iv, content string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	padded := pkcs7Pad([]byte(content), aes.BlockSize)
	encrypted := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, []byte(iv))
	mode.CryptBlocks(encrypted, padded)
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// AesDecrypt 对应 AesUtil.decrypt
func AesDecrypt(key, iv, content string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return "", err
	}
	if len(decoded) == 0 || len(decoded)%aes.BlockSize != 0 {
		return "", fmt.Errorf("invalid ciphertext length")
	}
	decrypted := make([]byte, len(decoded))
	mode := cipher.NewCBCDecrypter(block, []byte(iv))
	mode.CryptBlocks(decrypted, decoded)
	unpadded, err := pkcs7Unpad(decrypted, aes.BlockSize)
	if err != nil {
		return "", err
	}
	return string(unpadded), nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	return append(data, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("invalid padding")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize || padding > len(data) {
		return nil, fmt.Errorf("invalid padding")
	}
	return data[:len(data)-padding], nil
}

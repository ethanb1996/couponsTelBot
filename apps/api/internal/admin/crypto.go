package admin

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"strings"
)

func encryptCouponCode(key string, plainCode string) ([]byte, []byte, error) {
	if strings.TrimSpace(plainCode) == "" {
		return nil, nil, errors.New("coupon code is required")
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(strings.TrimSpace(plainCode)), nil)
	return ciphertext, nonce, nil
}

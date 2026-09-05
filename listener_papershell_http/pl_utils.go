package main

import (
	"crypto/rc4"
)

func RC4Crypt(data []byte, key []byte) ([]byte, error) {
	rc4crypt, errcrypt := rc4.NewCipher(key)
	if errcrypt != nil {
		return nil, errcrypt
	}
	decryptData := make([]byte, len(data))
	rc4crypt.XORKeyStream(decryptData, data)
	return decryptData, nil
}
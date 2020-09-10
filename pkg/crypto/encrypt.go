package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"
)

func randomBytes(length int) ([]byte, error) {
	buf := make([]byte, length)
	_, err := rand.Read(buf)
	return buf, err
}

// Generate an initialization vector for encryption.
// It consists of the current UNIX timestamp with nanoseconds, and four bytes of randomness.
func iv() ([]byte, error) {
	stor := make([]byte, 0)
	buf := bytes.NewBuffer(stor)

	err := binary.Write(buf, binary.BigEndian, time.Now().UnixNano())
	if err != nil {
		return nil, err
	}

	// Pad nonce with 4 bytes
	random, err := randomBytes(4)
	if err != nil {
		return nil, err
	}
	err = binary.Write(buf, binary.BigEndian, random)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Encrypts a plaintext with AES-256-GCM.
// Returns 12 bytes of IV, and then N bytes of ciphertext.
func Encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)

	nonce, err := iv()
	if err != nil {
		return nil, err
	}
	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)

	return append(nonce, ciphertext...), nil
}

// Decrypts a ciphertext encrypted with AES-256-GCM.
// The first 12 bytes of the ciphertext is assumed to be the IV.
func Decrypt(ciphertext, key []byte) ([]byte, error) {
	if len(ciphertext) <= 12 {
		return nil, fmt.Errorf("string is too short")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)

	return aesgcm.Open(nil, ciphertext[:12], ciphertext[12:], nil)
}

func KeyFromHexString(hexstr string) ([]byte, error) {
	encryptionKey, err := hex.DecodeString(hexstr)
	if err != nil {
		return nil, fmt.Errorf("encryption key must be a hex encoded string")
	}
	if len(encryptionKey) != 32 {
		return nil, fmt.Errorf("encryption key must be 256 bits; got %d", len(encryptionKey)*8)
	}

	return encryptionKey, nil
}

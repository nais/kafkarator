package certificate

import (
	"crypto/rand"
	"encoding/base64"
)

const (
	secretLength = 32
)

func GetSecret() (string, error) {
	b := make([]byte, secretLength)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

type StoreData struct {
	Keystore   []byte
	Truststore []byte
	Secret     string
}

type Generator interface {
	MakeStores(accessKey, accessCert, caCert string) (*StoreData, error)
}

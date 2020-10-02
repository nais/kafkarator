package certificate

import (
	"math/rand"
	"time"
)

const (
	secretLength = 32
	charset      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

func GetSecret() string {
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, secretLength)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

type StoreData struct {
	Keystore   []byte
	Truststore []byte
	Secret     string
}

type Generator interface {
	MakeStores(accessKey, accessCert, caCert string) (*StoreData, error)
}

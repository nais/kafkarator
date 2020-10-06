package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/Shopify/sarama"
	"k8s.io/client-go/util/keyutil"
)

type Message []byte

func TLSConfig(certificate, key, ca []byte) (*tls.Config, error) {
	cert, _ := pem.Decode(certificate)
	if cert == nil {
		return nil, fmt.Errorf("unable to parse certificate: no PEM data found")
	}

	k, err := keyutil.ParsePrivateKeyPEM(key)
	if err != nil {
		return nil, fmt.Errorf("unable to parse private key: %s", err)
	}

	cablock, _ := pem.Decode(ca)
	if cablock == nil {
		return nil, fmt.Errorf("unable to parse CA certificate: no PEM data found")
	}

	cacert, err := x509.ParseCertificate(cablock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse CA certificate: %s", err)
	}

	certpool := x509.NewCertPool()
	certpool.AddCert(cacert)

	return &tls.Config{
		Certificates: []tls.Certificate{
			{
				Certificate: [][]byte{cert.Bytes},
				PrivateKey:  k,
			},
		},
		RootCAs:            certpool,
		InsecureSkipVerify: false,
	}, nil
}

// Returns true if a sarama/kafka error is due to invalid credentials.
func IsErrUnauthorized(err error) bool {
	switch err.Error() {
	case sarama.ErrTopicAuthorizationFailed.Error():
		return true
	case sarama.ErrClusterAuthorizationFailed.Error():
		return true
	case sarama.ErrGroupAuthorizationFailed.Error():
		return true
	case sarama.ErrSASLAuthenticationFailed.Error():
		return true
	default:
		return false
	}
}

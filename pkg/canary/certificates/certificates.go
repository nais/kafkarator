package certificates

import (
	"bytes"

	"github.com/nais/kafkarator/pkg/utils"
	log "github.com/sirupsen/logrus"
)

type Certificates struct {
	certPath string
	keyPath  string
	caPath   string

	Cert []byte
	Key  []byte
	Ca   []byte
}

func New(certPath, keyPath, caPath string) (*Certificates, error) {
	cert, key, ca, err := utils.TlsFromFiles(certPath, keyPath, caPath)
	if err != nil {
		return nil, err
	}

	return &Certificates{
		certPath: certPath,
		keyPath:  keyPath,
		caPath:   caPath,

		Cert: cert,
		Key:  key,
		Ca:   ca,
	}, nil
}

func (c *Certificates) DiffAndUpdate(logger log.FieldLogger) {
	cert, _, _, err := utils.TlsFromFiles(c.certPath, c.keyPath, c.caPath)
	if err != nil {
		logger.Errorf("unable to read TLS config for diffing: %s", err)
		return
	}

	if bytes.Equal(c.Cert, cert) {
		logger.Debug("certificate on disk matches last read certificate")
		return
	}

	c.Cert = cert
	logger.Warnf("certificate changed on disk since last time it was read")
}

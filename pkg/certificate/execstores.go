package certificate

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
)

type ExecGenerator struct {
	secret string
	logger *log.Entry
}

func NewExecGenerator(logger *log.Logger) ExecGenerator {
	e := ExecGenerator{
		secret: GetSecret(),
		logger: log.NewEntry(logger),
	}
	return e
}

func (e ExecGenerator) MakeCredStores(accessKey, accessCert, caCert string) (*CredStoreData, error) {
	workdir, err := ioutil.TempDir("", "exec-store-workdir-*")
	defer os.RemoveAll(workdir)
	if err != nil {
		return nil, fmt.Errorf("failed to create workdir: %w", err)
	}
	keystore, err := e.MakeKeystore(workdir, accessKey, accessCert)
	if err != nil {
		return nil, err
	}
	truststore, err := e.MakeTruststore(workdir, caCert)
	if err != nil {
		return nil, err
	}
	return &CredStoreData{
		Keystore:   keystore,
		Truststore: truststore,
		Secret:     e.secret,
	}, nil
}

func (e ExecGenerator) MakeKeystore(workdir, accessKey, accessCert string) ([]byte, error) {
	keystorePath := path.Join(workdir, "client.keystore.p12")
	keyPath := path.Join(workdir, "access.key")
	err := ioutil.WriteFile(keyPath, []byte(accessKey), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write accessKey to temporary file: %w", err)
	}
	certPath := path.Join(workdir, "access.cert")
	err = ioutil.WriteFile(certPath, []byte(accessCert), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write accessCert to temporary file: %w", err)
	}
	cmd := exec.Command("openssl", "pkcs12",
		"-export",
		"-inkey", keyPath,
		"-in", certPath,
		"-out", keystorePath,
		"-passout", "stdin",
	)
	cmd.Stdin = strings.NewReader(e.secret)
	output, err := cmd.CombinedOutput()
	if err != nil {
		e.logger.Errorf("Failed to generate keystore! Output from command: \n%v", output)
		return nil, fmt.Errorf("failed to generate keystore: %w", err)
	}
	keystore, err := ioutil.ReadFile(keystorePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read keystore from generated file: %w", err)
	}
	return keystore, nil
}

func (e ExecGenerator) MakeTruststore(workdir, caCert string) ([]byte, error) {
	truststorePath := path.Join(workdir, "client.truststore.jks")
	caPath := path.Join(workdir, "ca.cert")
	err := ioutil.WriteFile(caPath, []byte(caCert), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write caCert to temporary file: %w", err)
	}
	cmd := exec.Command("keytool",
		"-importcert",
		"-noprompt",
		"-file", caPath,
		"-alias", "CA",
		"-keystore", truststorePath,
		"-storepass", e.secret,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		e.logger.Errorf("Failed to generate truststore! Output from command: \n%v", output)
		return nil, fmt.Errorf("failed to generate truststore: %w", err)
	}
	truststore, err := ioutil.ReadFile(truststorePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read truststore from generated file: %w", err)
	}
	return truststore, nil
}

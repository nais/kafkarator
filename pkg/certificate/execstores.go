package certificate

import (
	"io/ioutil"
	"os/exec"
	"path"
	"strings"
)

type ExecGenerator struct {
	secret string
}

func NewExecGenerator() ExecGenerator {
	e := ExecGenerator{
		secret: GetSecret(),
	}
	return e
}

func (e ExecGenerator) MakeStores(accessKey, accessCert, caCert string) (*StoreData, error) {
	workdir, err := ioutil.TempDir("", "exec-store-workdir-*")
	if err != nil {
		return nil, err
	}
	keystore, err := e.MakeKeystore(workdir, accessKey, accessCert)
	if err != nil {
		return nil, err
	}
	truststore, err := e.MakeTruststore(workdir, caCert)
	if err != nil {
		return nil, err
	}
	return &StoreData{
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
		return nil, err
	}
	certPath := path.Join(workdir, "access.cert")
	err = ioutil.WriteFile(certPath, []byte(accessCert), 0644)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("openssl", "pkcs12",
		"-export",
		"-inkey", keyPath,
		"-in", certPath,
		"-out", keystorePath,
		"-passout", "stdin",
	)
	cmd.Stdin = strings.NewReader(e.secret)
	err = cmd.Run()
	if err != nil {
		return nil, err
	}
	keystore, err := ioutil.ReadFile(keystorePath)
	if err != nil {
		return nil, err
	}
	return keystore, nil
}

func (e ExecGenerator) MakeTruststore(workdir, caCert string) ([]byte, error) {
	truststorePath := path.Join(workdir, "client.truststore.jks")
	caPath := path.Join(workdir, "ca.cert")
	err := ioutil.WriteFile(caPath, []byte(caCert), 0644)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("keytool",
		"-importcert",
		"-noprompt",
		"-file", caPath,
		"-alias", "CA",
		"-keystore", truststorePath,
		"-storepass", e.secret,
	)
	err = cmd.Run()
	if err != nil {
		return nil, err
	}
	truststore, err := ioutil.ReadFile(truststorePath)
	if err != nil {
		return nil, err
	}
	return truststore, nil
}

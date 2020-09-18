package utils

import (
	"fmt"
	"io/ioutil"
)

func TlsFromFiles(certPath, keyPath, caPath string) (cert, key, ca []byte, err error) {

	cert, err = ioutil.ReadFile(certPath)
	if err != nil {
		err = fmt.Errorf("unable to read certificate file %s: %s", certPath, err)
		return
	}

	key, err = ioutil.ReadFile(keyPath)
	if err != nil {
		err = fmt.Errorf("unable to read key file %s: %s", keyPath, err)
		return
	}

	ca, err = ioutil.ReadFile(caPath)
	if err != nil {
		err = fmt.Errorf("unable to read CA certificate file %s: %s", caPath, err)
		return
	}

	return
}

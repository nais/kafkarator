package utils

import (
	"fmt"
	"os"
)

func TlsFromFiles(certPath, keyPath, caPath string) (cert, key, ca []byte, err error) {

	cert, err = os.ReadFile(certPath)
	if err != nil {
		err = fmt.Errorf("unable to read certificate file %s: %s", certPath, err)
		return
	}

	key, err = os.ReadFile(keyPath)
	if err != nil {
		err = fmt.Errorf("unable to read key file %s: %s", keyPath, err)
		return
	}

	ca, err = os.ReadFile(caPath)
	if err != nil {
		err = fmt.Errorf("unable to read CA certificate file %s: %s", caPath, err)
		return
	}

	return
}

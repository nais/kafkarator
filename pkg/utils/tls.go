package utils

import (
	"fmt"
	"os"
)

func TlsFromFiles(certPath, keyPath, caPath string) ([]byte, []byte, []byte, error) {
	cert, err := os.ReadFile(certPath) // #nosec G304
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unable to read certificate file %s: %w", certPath, err)
	}

	key, err := os.ReadFile(keyPath) // #nosec G304
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unable to read key file %s: %w", keyPath, err)
	}

	ca, err := os.ReadFile(caPath) // #nosec G304
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unable to read CA certificate file %s: %w", caPath, err)
	}

	return cert, key, ca, nil
}

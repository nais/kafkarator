package certificate

func GetSecret() string {
	// This is typically the default password use "everywhere" for these kinds of stores
	return "changeme"
}

type CredStoreData struct {
	Keystore   []byte
	Truststore []byte
	Secret     string
}

type Generator interface {
	MakeCredStores(accessKey, accessCert, caCert string) (*CredStoreData, error)
}

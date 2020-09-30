package crypto

type manager struct {
	Key []byte
}

type Manager interface {
	Encrypt([]byte) ([]byte, error)
	Decrypt([]byte) ([]byte, error)
}

func NewManager(key []byte) Manager {
	return &manager{
		Key: key,
	}
}

func (c *manager) Encrypt(plaintext []byte) ([]byte, error) {
	return Encrypt(plaintext, c.Key)
}

func (c *manager) Decrypt(ciphertext []byte) ([]byte, error) {
	return Decrypt(ciphertext, c.Key)
}

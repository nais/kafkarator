package utils

import (
	"fmt"
	"hash/crc32"
)

// Procedurally generate a short string with hash that can be calculated using the base name
func ShortName(basename string, maxlen int) (string, error) {
	maxlen -= 9 // 8 bytes of hexadecimal hash and 1 byte of separator
	hasher := crc32.NewIEEE()
	_, err := hasher.Write([]byte(basename))
	if err != nil {
		return "", err
	}
	hashStr := fmt.Sprintf("%x", hasher.Sum32())
	if len(basename) > maxlen {
		basename = basename[:maxlen]
	}
	return basename + "-" + hashStr, nil
}

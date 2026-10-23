package plays

import "crypto/rand"

func fillRandom(b []byte) error {
	_, err := rand.Read(b)
	return err
}

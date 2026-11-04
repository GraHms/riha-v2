package crypto

import "golang.org/x/crypto/bcrypt"

type Bcrypt struct{}

const dummy = "$2a$10$7EqJtq98hPqEX7fNZaFWoOhi5BWX4Z1lFBn/iFQW6O0Uq3QzR8Zqi"

func (Bcrypt) Hash(p string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(b), err
}

func (Bcrypt) Matches(hash, p string) bool {
	if hash == "" {
		hash = dummy
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) == nil
}

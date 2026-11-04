package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strings"
)

const alphabet = "0123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
const upper = "0123456789ABCDEFGHJKLMNPQRSTUVWXYZ"

func randFrom(set string, n int) string {
	var b strings.Builder
	max := big.NewInt(int64(len(set)))
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic(err)
		}
		b.WriteByte(set[v.Int64()])
	}
	return b.String()
}

func NewID(prefix string) string { return prefix + "_" + randFrom(alphabet, 16) }

func ModeID(prefix string, live bool) string {
	return prefix + "_" + Mode(live) + "_" + randFrom(alphabet, 16)
}

func Token(prefix string, n int) string { return prefix + randFrom(alphabet, n) }

func PaymentReference() string { return "RH-" + randFrom(upper, 4) + "-" + randFrom("0123456789", 4) }

func ShortCode(n int) string { return randFrom(upper, n) }

func Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func Mode(live bool) string {
	if live {
		return "live"
	}
	return "test"
}

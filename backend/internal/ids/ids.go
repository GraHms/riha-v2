// Package ids generates prefixed, URL-safe random identifiers (pay_…, cs_test_…).
package ids

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

// New returns prefix + "_" + 16 random chars.
func New(prefix string) string { return prefix + "_" + randFrom(alphabet, 16) }

// Moded returns prefix_test_xxx or prefix_live_xxx.
func Moded(prefix string, live bool) string {
	m := "test"
	if live {
		m = "live"
	}
	return prefix + "_" + m + "_" + randFrom(alphabet, 16)
}

// Token returns a long secret token (for API keys, sessions, webhook secrets).
func Token(prefix string, n int) string { return prefix + randFrom(alphabet, n) }

// PaymentReference returns a human friendly reference such as RH-8K2D-4410.
func PaymentReference() string {
	return "RH-" + randFrom(upper, 4) + "-" + randFrom("0123456789", 4)
}

// Hash returns the sha256 hex digest of s.
func Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// Short returns n uppercase chars (used for operator references).
func Short(n int) string { return randFrom(upper, n) }

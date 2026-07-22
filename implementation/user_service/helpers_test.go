package userservice

import (
	"crypto/rand"
	"crypto/rsa"
)

// generatedRSAKey is a single 2048-bit RSA key generated once for the whole test
// binary (key generation is expensive; sharing keeps the suite fast).
var generatedRSAKey = mustGenerateRSAKey()

func mustGenerateRSAKey() *rsa.PrivateKey {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return key
}

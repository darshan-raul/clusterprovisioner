package jwks

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
)

// jwk is a minimal JSON Web Key decoder supporting the kty values
// Strata needs: RSA and EC. We deliberately avoid a third-party
// JOSE library; the format is simple enough.
type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func (j *jwk) kid() (string, error) {
	return j.Kid, nil
}

func (j *jwk) publicKey() any {
	switch j.Kty {
	case "RSA":
		return j.rsa()
	case "EC":
		return j.ec()
	}
	return nil
}

func (j *jwk) rsa() *rsa.PublicKey {
	nBytes, err := base64.RawURLEncoding.DecodeString(j.N)
	if err != nil {
		return nil
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(j.E)
	if err != nil {
		return nil
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}
}

func (j *jwk) ec() *ecdsa.PublicKey {
	xBytes, err := base64.RawURLEncoding.DecodeString(j.X)
	if err != nil {
		return nil
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(j.Y)
	if err != nil {
		return nil
	}
	var curve elliptic.Curve
	switch j.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil
	}
	return &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}
}

// parseJWK parses a single key JSON object. Phase 1 only needs RSA
// and EC; PSS keys land if Keycloak ever returns them.
func parseJWK(raw []byte) (*jwk, error) {
	var k jwk
	if err := json.Unmarshal(raw, &k); err != nil {
		return nil, err
	}
	if k.Kty == "" {
		return nil, errors.New("jwks: missing kty")
	}
	return &k, nil
}

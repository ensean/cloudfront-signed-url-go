package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"
)

// Signer holds the CloudFront signing credentials.
type Signer struct {
	keyPairID          string
	privateKey         *rsa.PrivateKey
	distributionDomain string
	defaultTTL         time.Duration
}

// NewSigner loads the RSA private key from disk and returns a ready Signer.
func NewSigner(cfg *Config) (*Signer, error) {
	keyData, err := os.ReadFile(cfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key %q: %w", cfg.PrivateKeyPath, err)
	}

	privKey, err := parsePEMPrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return &Signer{
		keyPairID:          cfg.KeyPairID,
		privateKey:         privKey,
		distributionDomain: cfg.DistributionDomain,
		defaultTTL:         time.Duration(cfg.DefaultTTLSeconds) * time.Second,
	}, nil
}

// Sign generates a CloudFront Canned Policy signed URL.
// s3Path is the object path, e.g. "/images/photo.jpg".
// If ttl <= 0, the configured default TTL is used.
func (s *Signer) Sign(s3Path string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	if !strings.HasPrefix(s3Path, "/") {
		s3Path = "/" + s3Path
	}

	resourceURL := fmt.Sprintf("https://%s%s", s.distributionDomain, s3Path)
	expires := time.Now().Add(ttl).Unix()

	// Canned Policy JSON (no whitespace — CloudFront is strict about this)
	policy := fmt.Sprintf(
		`{"Statement":[{"Resource":"%s","Condition":{"DateLessThan":{"AWS:EpochTime":%d}}}]}`,
		resourceURL, expires,
	)

	// Sign the policy with RSA-SHA1
	sig, err := rsaSHA1Sign(s.privateKey, []byte(policy))
	if err != nil {
		return "", fmt.Errorf("sign policy: %w", err)
	}

	signedURL := fmt.Sprintf("%s?Expires=%d&Signature=%s&Key-Pair-Id=%s",
		resourceURL,
		expires,
		cloudfrontBase64Encode(sig),
		s.keyPairID,
	)

	return signedURL, nil
}

// rsaSHA1Sign signs data with the given RSA private key using SHA-1.
func rsaSHA1Sign(key *rsa.PrivateKey, data []byte) ([]byte, error) {
	h := sha1.New()
	h.Write(data)
	digest := h.Sum(nil)
	return rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA1, digest)
}

// cloudfrontBase64Encode encodes bytes to CloudFront's variant of Base64:
// standard Base64 with +→-, =→_, /→~
func cloudfrontBase64Encode(b []byte) string {
	s := base64.StdEncoding.EncodeToString(b)
	s = strings.ReplaceAll(s, "+", "-")
	s = strings.ReplaceAll(s, "=", "_")
	s = strings.ReplaceAll(s, "/", "~")
	return s
}

// parsePEMPrivateKey decodes a PEM block and parses an RSA private key.
// Supports both PKCS#1 ("RSA PRIVATE KEY") and PKCS#8 ("PRIVATE KEY") formats.
func parsePEMPrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS#8 key is not RSA")
		}
		return rsaKey, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}
}

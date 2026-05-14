package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/cloudfront/sign"
)

// SDKSigner wraps the AWS SDK v2 CloudFront URL signer.
type SDKSigner struct {
	signer             *sign.URLSigner
	distributionDomain string
	defaultTTL         time.Duration
}

// NewSDKSigner loads the RSA private key from disk and returns a ready SDKSigner.
func NewSDKSigner(cfg *Config) (*SDKSigner, error) {
	keyData, err := os.ReadFile(cfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key %q: %w", cfg.PrivateKeyPath, err)
	}

	// reuse parsePEMPrivateKey from signer.go which handles both PKCS#1 and PKCS#8
	privKey, err := parsePEMPrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return &SDKSigner{
		signer:             sign.NewURLSigner(cfg.KeyPairID, privKey),
		distributionDomain: cfg.DistributionDomain,
		defaultTTL:         time.Duration(cfg.DefaultTTLSeconds) * time.Second,
	}, nil
}

// Sign generates a CloudFront Canned Policy signed URL via AWS SDK v2.
// s3Path is the object path, e.g. "/images/photo.jpg".
// If ttl <= 0, the configured default TTL is used.
func (s *SDKSigner) Sign(s3Path string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	if !strings.HasPrefix(s3Path, "/") {
		s3Path = "/" + s3Path
	}

	resourceURL := fmt.Sprintf("https://%s%s", s.distributionDomain, s3Path)
	expires := time.Now().Add(ttl)

	signedURL, err := s.signer.Sign(resourceURL, expires)
	if err != nil {
		return "", fmt.Errorf("sign URL: %w", err)
	}

	return signedURL, nil
}

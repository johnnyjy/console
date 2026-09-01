package util

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"
)

const (
	RsaPrivateKeyPemType = "RSA PRIVATE KEY"
	CertificatePemType   = "CERTIFICATE"
)

// GenerateRsaKeyPair 对应 CertificateUtil.generateRsaKeyPair
func GenerateRsaKeyPair(keySize int) (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, keySize)
}

// GenerateSelfSignedCertificate 对应 CertificateUtil.generateSelfSignedCertificate，
// 返回 DER 编码的证书内容。
func GenerateSelfSignedCertificate(keyPair *rsa.PrivateKey, host string, duration time.Duration) ([]byte, error) {
	notBefore := time.Now()
	notAfter := notBefore.Add(duration)

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: host},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
		DNSNames:              []string{host},
	}
	return x509.CreateCertificate(rand.Reader, &template, &template, &keyPair.PublicKey, keyPair)
}

// ToPem 对应 CertificateUtil.toPem
func ToPem(pemType string, content []byte) string {
	block := &pem.Block{Type: pemType, Bytes: content}
	return string(pem.EncodeToMemory(block))
}

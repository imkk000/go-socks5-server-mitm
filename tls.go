package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"sync"
	"time"
)

func generateCert(domain string, caKeyPair tls.Certificate, caX509 *x509.Certificate) (*tls.Certificate, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: domain},
		DNSNames:     []string{domain},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, caX509, &priv.PublicKey, caKeyPair.PrivateKey)
	if err != nil {
		return nil, err
	}

	return &tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}, nil
}

func getCert(domain string, caKeyPair tls.Certificate, caX509 *x509.Certificate) *tls.Certificate {
	if v, ok := certCache.Load(domain); ok {
		return v.(*tls.Certificate)
	}
	cert, _ := generateCert(domain, caKeyPair, caX509)
	certCache.Store(domain, cert)
	return cert
}

var (
	caCertPEM, _ = os.ReadFile("./cert/rootCA.pem")
	caKeyPEM, _  = os.ReadFile("./cert/rootCA-key.pem")
	caKeyPair, _ = tls.X509KeyPair(caCertPEM, caKeyPEM)
	caX509, _    = x509.ParseCertificate(caKeyPair.Certificate[0])
	certCache    = new(sync.Map)
)

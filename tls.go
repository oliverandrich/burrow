package burrow

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// tlsSetup holds the result of TLS configuration.
type tlsSetup struct {
	tlsConfig   *tls.Config  // nil means plain HTTP
	addr        string       // primary listen address
	httpHandler http.Handler // non-nil for ACME (:80 challenge+redirect)
	httpAddr    string       // ":80" for ACME, empty otherwise
}

// configureTLS resolves the TLS mode and returns the appropriate setup.
// ValidateTLS must be called before this function.
func configureTLS(cfg *Config) (*tlsSetup, error) {
	mode := cfg.resolvedTLSMode()

	switch mode {
	case "off":
		return configureTLSOff(cfg)
	case "selfsigned":
		return configureTLSSelfSigned(cfg)
	case "manual":
		return configureTLSManual(cfg)
	case "acme":
		return configureTLSACME(cfg)
	default:
		return nil, fmt.Errorf("unknown TLS mode: %q", cfg.TLS.Mode)
	}
}

func configureTLSOff(cfg *Config) (*tlsSetup, error) {
	return &tlsSetup{
		addr: fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
	}, nil
}

func configureTLSSelfSigned(cfg *Config) (*tlsSetup, error) {
	certFile := filepath.Join(cfg.TLS.CertDir, "selfsigned-cert.pem")
	keyFile := filepath.Join(cfg.TLS.CertDir, "selfsigned-key.pem")

	// Generate if missing.
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		if err := generateSelfSignedCert(cfg.Server.Host, certFile, keyFile); err != nil {
			return nil, fmt.Errorf("generate self-signed cert: %w", err)
		}
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load self-signed cert: %w", err)
	}

	return &tlsSetup{
		tlsConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		},
		addr: fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
	}, nil
}

func configureTLSManual(cfg *Config) (*tlsSetup, error) {
	cert, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load TLS cert: %w", err)
	}

	return &tlsSetup{
		tlsConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		},
		addr: fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
	}, nil
}

func configureTLSACME(cfg *Config) (*tlsSetup, error) {
	m := &autocert.Manager{
		Cache:      autocert.DirCache(cfg.TLS.CertDir),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(cfg.Server.Host),
		Email:      cfg.TLS.Email,
	}

	return &tlsSetup{
		tlsConfig:   m.TLSConfig(),
		addr:        ":443",
		httpHandler: m.HTTPHandler(nil),
		httpAddr:    ":80",
	}, nil
}

func generateSelfSignedCert(host, certFile, keyFile string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{Organization: []string{"Burrow Development"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	// Add SANs.
	if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
	} else {
		tmpl.DNSNames = append(tmpl.DNSNames, host)
	}
	tmpl.IPAddresses = append(tmpl.IPAddresses, net.ParseIP("127.0.0.1"), net.ParseIP("::1"))

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(certFile), 0o700); mkdirErr != nil {
		return mkdirErr
	}

	if writeErr := writePEMFile(certFile, "CERTIFICATE", certDER, 0o644); writeErr != nil {
		return writeErr
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}

	return writePEMFile(keyFile, "EC PRIVATE KEY", keyDER, 0o600)
}

func writePEMFile(path, blockType string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(filepath.Clean(path), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	encodeErr := pem.Encode(f, &pem.Block{Type: blockType, Bytes: data})
	closeErr := f.Close()
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}

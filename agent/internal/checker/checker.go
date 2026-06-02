package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"
)

type CheckResult struct {
	Status       string
	NotAfter     *time.Time
	Issuer       string
	Subject      string
	SANs         []string
	ErrorMessage string
}

func CheckDomain(ctx context.Context, host string, port int, protocol string) CheckResult {
	addr := fmt.Sprintf("%s:%d", host, port)
	dialer := &net.Dialer{}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	})
	if err != nil {
		return CheckResult{Status: "unreachable", ErrorMessage: err.Error()}
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return CheckResult{Status: "unreachable", ErrorMessage: "no certificate returned"}
	}
	leaf := certs[0]
	notAfter := leaf.NotAfter

	if err := leaf.VerifyHostname(host); err != nil {
		return CheckResult{
			Status: "mismatch", NotAfter: &notAfter,
			Issuer: leaf.Issuer.String(), Subject: leaf.Subject.String(), SANs: leaf.DNSNames,
			ErrorMessage: fmt.Sprintf("hostname verification failed: %v", err),
		}
	}
	if time.Now().After(leaf.NotAfter) {
		return CheckResult{
			Status: "expired", NotAfter: &notAfter,
			Issuer: leaf.Issuer.String(), Subject: leaf.Subject.String(), SANs: leaf.DNSNames,
		}
	}
	return CheckResult{
		Status: "ok", NotAfter: &notAfter,
		Issuer: leaf.Issuer.String(), Subject: leaf.Subject.String(), SANs: leaf.DNSNames,
	}
}

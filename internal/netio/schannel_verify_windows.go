package netio

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// chainEngine is the chain engine a server's certificate is verified by: zero, the current user's,
// which is what crypto/x509 asked on Windows, unless a test has made one that trusts its own server
// alone. No revocation is checked, as crypto/x509 checked none.
var chainEngine windows.Handle

// verify asks the system whether the certificate the server sent, with the certificates sent
// beside it, chains to a root it trusts, is for a server, and is serverName's.
func (c *schannelConn) verify(serverName string) error {
	var certificate *windows.CertContext
	if status, _, _ := sspi.query.Call(uintptr(unsafe.Pointer(&c.context)), attributeRemoteCert, uintptr(unsafe.Pointer(&certificate))); status != secOK {
		return schannelError("no certificate", status)
	}
	defer windows.CertFreeCertificateContext(certificate)
	serverAuth := append([]byte("1.3.6.1.5.5.7.3.1"), 0) // szOID_PKIX_KP_SERVER_AUTH
	usage := &serverAuth[0]
	para := windows.CertChainPara{RequestedUsage: windows.CertUsageMatch{
		Type: windows.USAGE_MATCH_TYPE_OR, Usage: windows.CertEnhKeyUsage{Length: 1, UsageIdentifiers: &usage}}}
	para.Size = uint32(unsafe.Sizeof(para))
	const lowerQuality = 0x80 // CERT_CHAIN_RETURN_LOWER_QUALITY_CONTEXTS, as crypto/x509 asked
	var chain *windows.CertChainContext
	if err := windows.CertGetCertificateChain(chainEngine, certificate, nil, certificate.Store, &para, lowerQuality, 0, &chain); err != nil {
		return fmt.Errorf("certificate: %w", err)
	}
	defer windows.CertFreeCertificateChain(chain)
	if err := untrusted(chain.TrustStatus.ErrorStatus); err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(serverName)
	if err != nil {
		return err
	}
	extra := windows.SSLExtraCertChainPolicyPara{AuthType: windows.AUTHTYPE_SERVER, ServerName: name}
	extra.Size = uint32(unsafe.Sizeof(extra))
	policy := windows.CertChainPolicyPara{ExtraPolicyPara: (windows.Pointer)(unsafe.Pointer(&extra))}
	policy.Size = uint32(unsafe.Sizeof(policy))
	var status windows.CertChainPolicyStatus
	status.Size = uint32(unsafe.Sizeof(status))
	if err := windows.CertVerifyCertificateChainPolicy(windows.CERT_CHAIN_POLICY_SSL, chain, &policy, &status); err != nil {
		return fmt.Errorf("certificate: %w", err)
	}
	switch status.Error {
	case 0:
		return nil
	case uint32(windows.CERT_E_CN_NO_MATCH):
		return fmt.Errorf("certificate is not valid for %s", serverName)
	}
	return fmt.Errorf("certificate refused: (%#x) %s", status.Error, windows.Errno(status.Error).Error())
}

// untrusted is the reason a chain's status gives for not trusting it, crypto/x509's where it had
// one.
func untrusted(status uint32) error {
	switch {
	case status == windows.CERT_TRUST_NO_ERROR:
		return nil
	case status&windows.CERT_TRUST_IS_NOT_TIME_VALID != 0:
		return errors.New("certificate has expired or is not yet valid")
	case status&windows.CERT_TRUST_IS_REVOKED != 0:
		return errors.New("certificate has been revoked")
	case status&windows.CERT_TRUST_IS_NOT_VALID_FOR_USAGE != 0:
		return errors.New("certificate is not for a server")
	case status&(windows.CERT_TRUST_IS_UNTRUSTED_ROOT|windows.CERT_TRUST_IS_PARTIAL_CHAIN) != 0:
		return errors.New("certificate signed by unknown authority")
	}
	return fmt.Errorf("certificate not trusted: chain status %#x", status)
}

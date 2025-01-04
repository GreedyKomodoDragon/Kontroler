package webhook

type SSLVerifier interface {
	VerifySSL(url string) error
}

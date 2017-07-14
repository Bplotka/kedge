package tripperware

import (
	"crypto/tls"
	"net/http"
	"strings"
)

type RoundTripper interface {
	http.RoundTripper
	Clone() RoundTripper
}

type defaultTripper struct {
	*http.Transport
}

func (t *defaultTripper) Clone() RoundTripper {
	return &(*t)
}

func Default(config *tls.Config) RoundTripper {
	transport := http.DefaultTransport.(*http.Transport)
	transport.TLSClientConfig = config
	return &defaultTripper{Transport: transport}
}

func DefaultWithTransport(transport *http.Transport, config *tls.Config) RoundTripper {
	transport.TLSClientConfig = config
	return &defaultTripper{Transport: transport}
}

func getURLHost(req *http.Request) string {
	host := req.URL.Host
	if strings.Contains(host, ":") {
		host = host[:strings.LastIndex(host, ":")]
	}
	return host
}
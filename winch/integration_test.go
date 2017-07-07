// Integration tests for winch.
package winch_test

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/mwitkow/go-conntrack/connhelpers"
	pb "github.com/mwitkow/kedge/_protogen/winch/config"
	"github.com/mwitkow/kedge/lib/map"
	"github.com/mwitkow/kedge/winch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func unknownPingbackHandler(id int) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("x-test-req-proto", fmt.Sprintf("%d.%d", req.ProtoMajor, req.ProtoMinor))
		resp.Header().Set("x-test-req-url", req.URL.String())
		resp.Header().Set("x-test-req-host", req.Host)
		resp.Header().Set("x-test-kedge-id", strconv.Itoa(id))
		resp.Header().Set("x-test-auth-value", req.Header.Get("Authorization"))
		resp.WriteHeader(http.StatusAccepted) // accepted to make sure stuff is slightly different.
	})
}

type localKedges struct {
	listeners []net.Listener
	servers   []*http.Server
}

func buildAndStartServer(t *testing.T, config *tls.Config, index int) (net.Listener, *http.Server) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "must be able to allocate a port for local kedge")
	if config != nil {
		listener = tls.NewListener(listener, config)
	}
	server := &http.Server{
		// TODO(bplotka): Mimic OIDC support when added on kedge.
		Handler: unknownPingbackHandler(index),
	}
	go func() {
		server.Serve(listener)
	}()
	return listener, server
}

func (l *localKedges) SetupKedges(t *testing.T, config *tls.Config, num int) {
	for i := 0; i < num; i++ {
		listener, server := buildAndStartServer(t, config, i)
		l.servers = append(l.servers, server)
		l.listeners = append(l.listeners, listener)
	}
}

func (l *localKedges) Close() error {
	for _, l := range l.listeners {
		l.Close()
	}
	return nil
}

// WinchIntegrationSuite performs tests that simulates following setup:
// client <- plain  HTTP -> winch (forward proxy) <- TLS -> kedge (reverse proxy)
// Kedge configuration is mimicked as OIDC authed kedge, so ClientAuth is set only to tls.VerifyClientCertIfGiven
type WinchIntegrationSuite struct {
	suite.Suite

	winch              *http.Server
	winchListenerPlain net.Listener
	routes             *winch.StaticRoutes

	// Will be used to call winch.
	winchMapper kedge_map.Mapper

	localSecureKedges localKedges
}

func moveToLocalhost(addr string) string {
	return strings.Replace(addr, "127.0.0.1", "localhost", -1)
}

func TestBackendPoolIntegrationTestSuite(t *testing.T) {
	suite.Run(t, &WinchIntegrationSuite{})
}

func (s *WinchIntegrationSuite) SetupSuite() {
	var err error
	s.winchListenerPlain, err = net.Listen("tcp", "localhost:0")
	require.NoError(s.T(), err, "must be able to allocate a port for winchListenerPlain")

	http2ServerTlsConfig, err := connhelpers.TlsConfigWithHttp2Enabled(s.tlsServerConfigForTest())
	if err != nil {
		s.FailNow("cannot configure the tls config for http2")
	}

	// It does not make sense if kedge is not secure.
	s.localSecureKedges.SetupKedges(s.T(), http2ServerTlsConfig, 3)

	testConfig := &pb.MapperConfig{
		Routes: []*pb.Route{
			{
				Type: &pb.Route_Direct{
					Direct: &pb.DirectRoute{
						Key:      "resource1.ext.example.com",
						KedgeUrl: "https://" + moveToLocalhost(s.localSecureKedges.listeners[0].Addr().String()),
					},
				},
			},
			{
				Type: &pb.Route_Direct{
					Direct: &pb.DirectRoute{
						Key:      "resource2.ext.example.com",
						KedgeUrl: "https://" + moveToLocalhost(s.localSecureKedges.listeners[1].Addr().String()),
					},
				},
			},
			{
				Type: &pb.Route_Regexp{
					Regexp: &pb.RegexpRoute{
						Exp:              "([a-z0-9-].*).(?P<cluster>[a-z0-9-].*).internal.example.com",
						ClusterGroupName: "cluster",
						KedgeUrl:         "https://" + moveToLocalhost(s.localSecureKedges.listeners[2].Addr().String()),
					},
				},
			},
		},
	}
	s.routes, err = winch.NewStaticRoutes(testConfig)
	require.NoError(s.T(), err, "config must be parsable")

	pacHandler, err := winch.NewPac(s.winchListenerPlain.Addr().String(), testConfig)
	require.NoError(s.T(), err, "pac should init")

	assert.Equal(s.T(), expectedPAC(s.winchListenerPlain.Addr().String()), string(pacHandler.PAC))

	http.Handle("/pac", pacHandler)
	http.Handle("/", winch.New(kedge_map.RouteMapper(s.routes), s.tlsClientConfigForTest()))
	s.winch = &http.Server{
		Handler: http.DefaultServeMux,
	}
	go func() {
		s.winch.Serve(s.winchListenerPlain)
	}()
}

func (s *WinchIntegrationSuite) TearDownSuite() {
	if s.winch != nil {
		s.winchListenerPlain.Close()
	}
	s.localSecureKedges.Close()
}

func (s *WinchIntegrationSuite) assertSuccessfulPingback(req *http.Request, resp *http.Response, err error, kedgeID int) {
	require.NoError(s.T(), err, "no error on a call to a winch")
	assert.Empty(s.T(), resp.Header.Get("x-kedge-error"))
	require.Equal(s.T(), http.StatusAccepted, resp.StatusCode)
	assert.Equal(s.T(), req.URL.Path, resp.Header.Get("x-test-req-url"), "path seen on kedge must match requested path")
	assert.Equal(s.T(), strconv.Itoa(kedgeID), resp.Header.Get("x-test-kedge-id"), "expected kedge must respond to our request")
}

func (s *WinchIntegrationSuite) assertBadGatewayPingback(req *http.Request, resp *http.Response, err error) {
	require.NoError(s.T(), err, "no error on a call to a winch")
	assert.Empty(s.T(), resp.Header.Get("x-kedge-error"))
	require.Equal(s.T(), http.StatusBadGateway, resp.StatusCode)
	assert.Empty(s.T(), resp.Header.Get("x-test-req-url"))
	assert.Empty(s.T(), resp.Header.Get("x-test-kedge-id"))
}

func (s *WinchIntegrationSuite) TestCallKedgeThroughWinch_NoRoute() {
	req := &http.Request{Method: "GET", URL: urlMustParse("http://resourceXXX.ext.example.com/some/strict/path")}
	resp, err := s.forwardProxyClient().Do(req)
	s.assertBadGatewayPingback(req, resp, err)
}

func (s *WinchIntegrationSuite) TestCallKedgeThroughWinch_DirectRoute() {
	req := &http.Request{Method: "GET", URL: urlMustParse("http://resource1.ext.example.com/some/strict/path")}
	resp, err := s.forwardProxyClient().Do(req)
	s.assertSuccessfulPingback(req, resp, err, 0)
}

func (s *WinchIntegrationSuite) TestCallKedgeThroughWinch_DirectRoute2() {
	req := &http.Request{Method: "GET", URL: urlMustParse("http://resource2.ext.example.com/some/strict/path")}
	resp, err := s.forwardProxyClient().Do(req)
	s.assertSuccessfulPingback(req, resp, err, 1)
}

func (s *WinchIntegrationSuite) TestCallKedgeThroughWinch_RegexpRoute() {
	req := &http.Request{Method: "GET", URL: urlMustParse("http://service1.ab1-prod.internal.example.com/some/strict/path")}
	resp, err := s.forwardProxyClient().Do(req)
	s.assertSuccessfulPingback(req, resp, err, 2)
}

func (s *WinchIntegrationSuite) TestCallKedgeThroughWinch_RegexpRoute_PreserveAuthHeader() {
	req := &http.Request{Method: "GET", URL: urlMustParse("http://service1.ab1-prod.internal.example.com/some/strict/path")}
	req.Header = http.Header{}
	req.Header.Set("Authorization", "bearer test-secret")
	resp, err := s.forwardProxyClient().Do(req)
	s.assertSuccessfulPingback(req, resp, err, 2)
	assert.Equal(s.T(), "bearer test-secret", resp.Header.Get("x-test-auth-value"))
}

// Client that will proxy through winch.
func (s *WinchIntegrationSuite) forwardProxyClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			// This will make all dials over the Proxy mechanism. For "http" schemes it will used FORWARD_PROXY semantics.
			// For "https" scheme it will use CONNECT proxy. (Not supported currently).
			Proxy: func(req *http.Request) (*url.URL, error) {
				return urlMustParse("http://" + s.winchListenerPlain.Addr().String()), nil
			},
		},
	}
}

func (s *WinchIntegrationSuite) tlsServerConfigForTest() *tls.Config {
	tlsConfig, err := connhelpers.TlsConfigForServerCerts(
		path.Join(getTestingCertsPath(), "localhost.crt"),
		path.Join(getTestingCertsPath(), "localhost.key"))
	if err != nil {
		require.NoError(s.T(), err, "failed reading server certs")
	}
	tlsConfig.RootCAs = x509.NewCertPool()
	// Make Client cert verification an option.
	tlsConfig.ClientCAs = x509.NewCertPool()
	tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven
	data, err := ioutil.ReadFile(path.Join(getTestingCertsPath(), "ca.crt"))
	if err != nil {
		s.FailNow("Failed reading CA: %v", err)
	}
	if ok := tlsConfig.RootCAs.AppendCertsFromPEM(data); !ok {
		s.FailNow("failed processing CA file")
	}
	if ok := tlsConfig.ClientCAs.AppendCertsFromPEM(data); !ok {
		s.FailNow("failed processing CA file")
	}
	return tlsConfig
}

func (s *WinchIntegrationSuite) tlsClientConfigForTest() *tls.Config {
	tlsConfig := new(tls.Config)
	tlsConfig.RootCAs = x509.NewCertPool()
	// Make Client cert verification an option.
	data, err := ioutil.ReadFile(path.Join(getTestingCertsPath(), "ca.crt"))
	if err != nil {
		s.FailNow("Failed reading CA: %v", err)
	}
	if ok := tlsConfig.RootCAs.AppendCertsFromPEM(data); !ok {
		s.FailNow("failed processing CA file")
	}
	return tlsConfig
}

func getTestingCertsPath() string {
	_, callerPath, _, _ := runtime.Caller(0)
	return path.Join(path.Dir(callerPath), "..", "misc")
}

func urlMustParse(uStr string) *url.URL {
	u, err := url.Parse(uStr)
	if err != nil {
		panic(err)
	}
	return u
}

func expectedPAC(winchHostPort string) string {
	return fmt.Sprintf(`function FindProxyForURL(url, host) {
	var proxy = "PROXY %s; DIRECT";
    var direct = "DIRECT";

	// no proxy for local hosts without domain:
    if(isPlainHostName(host)) return direct;

	// We only proxy http, not even https.
     if (
         url.substring(0, 4) == "ftp:" ||
         url.substring(0, 6) == "rsync:" ||
         url.substring(0, 6) == "https:"
        )
    return direct;

	// Commented for debug purposes.
  	// Use direct connection whenever we have direct network connectivity.
    //if (isResolvable(host)) {
    //    return direct
    //}
    if (dnsDomainIs(host, resource1.ext.example.com)) {
        return proxy;
    }
    if (dnsDomainIs(host, resource2.ext.example.com)) {
        return proxy;
    }
    if (shExpMatch(host, ([a-z0-9-].*).(?P<cluster>[a-z0-9-].*).internal.example.com)) {
        return proxy;
    }

    return direct;
}`, winchHostPort)
}

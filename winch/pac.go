package winch

import (
	"bytes"
	"text/template"
	"net/http"

	"github.com/mwitkow/go-httpwares/tags"
	pb_config "github.com/mwitkow/kedge/_protogen/winch/config"
)

func NewPac(winchHostPort string, config *pb_config.MapperConfig) (*Pac, error) {
	pac, err := generatePAC(winchHostPort, config)
	if err != nil {
		return nil, err
	}
	p := &Pac{
		PAC: pac,
	}
	return p, nil
}

// Pac is a handler that serves auto generated PAC file based on mapping routes.
type Pac struct {
	PAC []byte
}

func (p *Pac) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	// TODO(bplotka): Pass only local connections.

	tags := http_ctxtags.ExtractInbound(req)
	tags.Set(http_ctxtags.TagForCallService, "Pac")

}

var (
	pacTemplate = `function FindProxyForURL(url, host) {
	var proxy = "PROXY {{.WinchHostPort}}; DIRECT";
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

    {{- range .Routes}}
	{{- if .GetDirect }}
    if (dnsDomainIs(host, {{ .GetDirect.Key }})) {
        return proxy;
    }
	{{- end }}
	{{- if .GetRegexp }}
    if (shExpMatch(host, {{ .GetRegexp.Exp }})) {
        return proxy;
    }
	{{- end }}
    {{- end}}

    return direct;
}`
)

func generatePAC(winchHostPort string, config *pb_config.MapperConfig) ([]byte, error) {
	tmpl, err := template.New("PAC").Parse(pacTemplate)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, struct {
		WinchHostPort string
		Routes        []*pb_config.Route
	}{
		WinchHostPort: winchHostPort,
		Routes:        config.Routes,
	})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

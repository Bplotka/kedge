package main

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"github.com/mwitkow/go-conntrack"
	"github.com/mwitkow/go-flagz"
	"github.com/mwitkow/go-httpwares/logging/logrus"
	"github.com/mwitkow/go-httpwares/tags"
	"github.com/mwitkow/go-httpwares/tracing/debug"
	"github.com/mwitkow/kedge/lib/sharedflags"
	"github.com/pressly/chi"
	log "github.com/sirupsen/logrus"
	_ "golang.org/x/net/trace" // so /debug/request gets registered.
)

var (
	flagHttpPort = sharedflags.Set.Int("server_http_port", 8070, "TCP port to listen on for HTTP1.1/REST calls.")

	flagHttpMaxWriteTimeout = sharedflags.Set.Duration("server_http_max_write_timeout", 10*time.Second, "HTTP server config, max write duration.")
	flagHttpMaxReadTimeout  = sharedflags.Set.Duration("server_http_max_read_timeout", 10*time.Second, "HTTP server config, max read duration.")
)

func main() {
	if err := sharedflags.Set.Parse(os.Args); err != nil {
		log.Fatalf("failed parsing flags: %v", err)
	}
	if err := flagz.ReadFileFlags(sharedflags.Set); err != nil {
		log.Fatalf("failed reading flagz from files: %v", err)
	}

	log.SetOutput(os.Stdout)
	logEntry := log.NewEntry(log.StandardLogger())

	http.Handle("/debug/flagz", http.HandlerFunc(flagz.NewStatusEndpoint(sharedflags.Set).ListFlags))
	http.Handle("/", )
	winchServer := &http.Server{
		WriteTimeout: *flagHttpMaxWriteTimeout,
		ReadTimeout:  *flagHttpMaxReadTimeout,
		ErrorLog:     http_logrus.AsHttpLogger(logEntry),
		Handler: chi.Chain(
			http_ctxtags.Middleware("winch"),
			http_debug.Middleware(),
			http_logrus.Middleware(logEntry),
		).Handler(http.DefaultServeMux),
	}

	var httpPlainListener net.Listener
	httpPlainListener = buildListenerOrFail("http_plain", *flagHttpPort)
	log.Infof("listening for HTTP Plain on: %v", httpPlainListener.Addr().String())

	// Serve.
	if err := winchServer.Serve(httpPlainListener); err != nil {
		log.Fatalf("http_plain server error: %v", err)
	}
}

func buildListenerOrFail(name string, port int) net.Listener {
	addr := fmt.Sprintf("%s:%d", "127.0.0.1", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed listening for '%v' on %v: %v", name, port, err)
	}
	return conntrack.NewListener(listener,
		conntrack.TrackWithName(name),
		conntrack.TrackWithTcpKeepAlive(20*time.Second),
		conntrack.TrackWithTracing(),
	)
}

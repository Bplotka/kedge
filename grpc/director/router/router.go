package router

import (
	pb "github.com/mwitkow/kedge/_protogen/kedge/config/grpc/routes"

	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

var (
	emptyMd       = metadata.Pairs()
	routeNotFound = grpc.Errorf(codes.Unimplemented, "unknown route to service")
)

type Router interface {
	// Route returns a backend name for a given call, or an error.
	Route(ctx context.Context, fullMethodName string) (backendName string, err error)
}

type router struct {
	routes []*pb.Route
}

func NewStatic(routes []*pb.Route) *router {
	return &router{routes: routes}
}

func (r *router) Route(ctx context.Context, fullMethodName string) (backendName string, err error) {
	md := metautils.ExtractIncoming(ctx)
	if strings.HasPrefix(fullMethodName, "/") {
		fullMethodName = fullMethodName[1:]
	}
	for _, route := range r.routes {
		if !r.serviceNameMatches(fullMethodName, route.ServiceNameMatcher) {
			continue
		}
		if !r.authorityMatches(md, route.AuthorityMatcher) {
			continue
		}
		if !r.metadataMatches(md, route.MetadataMatcher) {
			continue
		}
		return route.BackendName, nil
	}
	return "", routeNotFound
}

func (r *router) serviceNameMatches(fullMethodName string, matcher string) bool {
	if matcher == "" || matcher == "*" {
		return true
	}
	if matcher[len(matcher)-1] == '*' {
		return strings.HasPrefix(fullMethodName, matcher[0:len(matcher)-1])
	}
	return fullMethodName == matcher
}

func (r *router) authorityMatches(md metautils.NiceMD, matcher string) bool {
	if matcher == "" {
		return true
	}
	auth := md.Get(":authority")
	if auth == "" {
		return false // there was no authority header and it was expected
	}
	return auth == matcher
}

func (r *router) metadataMatches(md metautils.NiceMD, expectedKv map[string]string) bool {
	for expK, expV := range expectedKv {
		vals, ok := md[strings.ToLower(expK)]
		if !ok {
			return false // key doesn't exist
		}
		found := false
		for _, v := range vals {
			if v == expV {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

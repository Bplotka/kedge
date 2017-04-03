package resolvers

import (
	"time"

	"github.com/mwitkow/go-srvlb/grpc"
	"github.com/mwitkow/go-srvlb/srv"
	pb "github.com/mwitkow/kedge/_protogen/kedge/config/common/resolvers"
	"google.golang.org/grpc/naming"
)

var (
	ParentSrvResolver = srv.NewGoResolver(5 * time.Second)
)

func NewSrvFromConfig(conf *pb.SrvResolver) (target string, namer naming.Resolver, err error) {
	return conf.GetDnsName(), grpcsrvlb.New(ParentSrvResolver), nil
}

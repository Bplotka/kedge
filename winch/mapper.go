package winch

import (
	"net/url"
	"github.com/mwitkow/kedge/lib/map"
	"github.com/mwitkow/kedge/lib/sharedflags"
)

var (
	flagRegexp  = sharedflags.Set.String("winch_regexp", "", "Regexp expression for ")
)

// Single is a simplistic kedge mapper that forwards all traffic through the same kedge.
func NewMapper() kedge_map.Mapper {
	return &single{kedgeUrl}
}

type mapper struct {
	kedgeUrl *url.URL
}

// TODO(bplotka): Check if that logic makes sense.
func (s *mapper) Map(targetAuthorityDnsName string) (*url.URL, error) {



	return s.kedgeUrl, nil
}

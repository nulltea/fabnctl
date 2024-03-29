package helm

import (
	"github.com/mittwald/go-helm-client"
	"github.com/timoth-y/fabnctl/pkg/term"
)

// Client defines shared client interface for Client cli.
var Client helmclient.Client

func init() {
	var (
		err error
		opt = &helmclient.Options{
			Debug:   true,
			Linting: true,
			DebugLog: func(format string, v ...interface{}) {
				term.NewLogger().StreamTextf(format, v...)
			},
		}
	)

	Client, err = helmclient.New(opt)
	if err != nil {
		panic(err)
	}
}

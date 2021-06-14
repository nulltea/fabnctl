package shared

import (
	"github.com/mittwald/go-helm-client"
)

// Helm defines shared client interface for Helm cli.
var Helm helmclient.Client

func initHelm() {
	var (
		err error
		opt = &helmclient.Options{
			Debug:            true,
			Linting:          true,
		}
	)

	Helm, err = helmclient.New(opt)
	if err != nil {
		panic(err)
	}
}

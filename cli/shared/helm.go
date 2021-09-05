package shared

import (
	"fmt"

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
			DebugLog: func(format string, v ...interface{}) {
				ILogger.Text("Helm: " + fmt.Sprintf(format, v...))
			},
		}
	)

	Helm, err = helmclient.New(opt)
	if err != nil {
		panic(err)
	}
}

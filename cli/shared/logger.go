package shared

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/gernest/wow"
	"github.com/gernest/wow/spin"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
)

// Logger is an instance of the shared logger tool.
var (
	Logger            *logging.Logger
	InteractiveLogger *wow.Wow
)

const (
	format = "%{color}%{time:2006.01.02 15:04:05} " +
		"%{id:04x} %{level:.4s}%{color:reset} " +
		"[%{module}] %{color:bold}%{shortfunc}%{color:reset} -> %{message}"
)

func initLogger() {
	var (
		envLevel = viper.GetString("logging")
		chaincodeName = viper.GetString("name")
	)

	Logger = logging.MustGetLogger(chaincodeName)

	backend := logging.NewBackendFormatter(
		logging.NewLogBackend(os.Stderr, "", 0),
		logging.MustStringFormatter(format),
	)

	level, err := logging.LogLevel(envLevel); if err != nil {
		level = logging.DEBUG
	}

	logging.SetBackend(backend)
	logging.SetLevel(level, chaincodeName)

	log.SetOutput(ioutil.Discard)
	InteractiveLogger = wow.New(os.Stderr, spin.Get(spin.Dots), "")
}

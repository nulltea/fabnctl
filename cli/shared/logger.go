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

var (
	// Logger is an instance of the shared logger tool.
	Logger            *logging.Logger
	// ILogger is an instance of the interactive logger.
	ILogger *wow.Wow
)

type ILogLevel int

const (
	ILogSuccess ILogLevel = iota
	ILogOk
	ILogError
	ILogWarning
	ILogInfo
)

var (
	ILogPrefixes map[ILogLevel]spin.Spinner
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
	ILogger = wow.New(os.Stderr, spin.Get(spin.Dots), "")
	ILogPrefixes = map[ILogLevel]spin.Spinner{
		ILogSuccess: {Frames: []string{viper.GetString("cli.success_emoji")}},
		ILogOk:      {Frames: []string{viper.GetString("cli.ok_emoji")}},
		ILogError:   {Frames: []string{viper.GetString("cli.error_emoji")}},
		ILogWarning: {Frames: []string{viper.GetString("cli.warning_emoji")}},
		ILogInfo:    {Frames: []string{viper.GetString("cli.info_emoji")}},
	}
}

// DecorateWithInteractiveLog wraps `fn` call into interactive logging with loading,
// displaying `start` message on loading, `complete` on successful end,
// and err return value on failure.
func DecorateWithInteractiveLog(fn func() error, start, complete string) error {
	ILogger.Start()
	defer ILogger.Stop()

	ILogger.Text(" " + start)
	if err := fn(); err != nil {
		ILogger.PersistWith(ILogPrefixes[ILogError], " " + err.Error())
		return err
	}

	ILogger.PersistWith(ILogPrefixes[ILogSuccess], " " + complete)

	return nil
}

// DecorateWithInteractiveLogWithPersist wraps `fn` call into interactive logging with loading,
// displaying `start` message on loading and custom persist on end.
func DecorateWithInteractiveLogWithPersist(fn func() (level ILogLevel, msg string), start string) {
	ILogger.Start()
	defer ILogger.Stop()

	ILogger.Text(" " + start)
	level, msg := fn()
	ILogger.PersistWith(ILogPrefixes[level], " " + msg)
}

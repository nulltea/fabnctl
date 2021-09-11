package term

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/gernest/wow"
	"github.com/gernest/wow/spin"
	"github.com/morikuni/aec"
	"github.com/spf13/viper"
	"k8s.io/kubectl/pkg/cmd/util"
)

type Logger struct {
	*loggerArgs
	streamer       *wow.Wow
	streamSpinners map[LogStreamLevel]spin.Spinner
}

func NewLogger(options ...LoggerOption) *Logger {
	var args = &loggerArgs{
		stdout: os.Stdout,
		stderr: os.Stderr,
	}

	for i := range options {
		options[i](args)
	}

	return &Logger{
		loggerArgs: args,
		streamer: wow.New(args.stderr, spin.Get(spin.Monkey), ""),
		streamSpinners: map[LogStreamLevel]spin.Spinner{
			LogStreamSuccess: {Frames: []string{viper.GetString("cli.success_emoji")}},
			LogStreamOk:      {Frames: []string{viper.GetString("cli.ok_emoji")}},
			LogStreamError:   {Frames: []string{viper.GetString("cli.error_emoji")}},
			LogStreamWarning: {Frames: []string{viper.GetString("cli.warning_emoji")}},
			LogStreamInfo:    {Frames: []string{viper.GetString("cli.info_emoji")}},
		},
	}
}

func (l *Logger) Success(message string) {
	_, _ = fmt.Fprintln(l.stdout, aec.GreenF,
		message, aec.DefaultF,
	)
}

func (l *Logger) Successf(format string, a ...interface{}) {
	l.Success(fmt.Sprintf(format, a...))
}

func (l *Logger) Info(message string) {
	_, _ = fmt.Fprintln(l.stdout, message)
}

func (l *Logger) Infof(format string, a ...interface{}) {
	l.Info(fmt.Sprintf(format, a...))
}

func (l *Logger) Ok(message string) {
	_, _ = fmt.Fprintln(l.stdout, message)
}

func (l *Logger) Okf(format string, a ...interface{}) {
	l.Ok(fmt.Sprintf(format, a...))
}

func (l *Logger) Errorf(format string, a ...interface{}) {
	_, _ = fmt.Fprintln(l.stderr, aec.LightRedF,
		fmt.Sprintf(format, a...), aec.DefaultF,
	)
}

func (l *Logger) Error(err error, message string) {
	if err != nil {
		_, _ = fmt.Fprintln(l.stderr, aec.LightRedF,
			fmt.Sprintf("%s: %v", message, err), aec.DefaultF,
		)
	}
}

func (l *Logger) MultiError(prefix string, errs ...error) {
	l.Error(fmt.Errorf(util.MultipleErrors(prefix, errs)), "multiple errors")
}

func (l *Logger) NewLine() {
	_, _ = fmt.Fprintln(l.stdout)
}


func init() {
	log.SetOutput(ioutil.Discard)
}

package term

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/gernest/wow"
	"github.com/gernest/wow/spin"
	"github.com/morikuni/aec"
	"github.com/spf13/viper"
	"k8s.io/kubectl/pkg/cmd/util"
)

type Logger struct {
	*loggerArgs
	Streamer       *wow.Wow
	StreamSpinners map[LogStreamLevel]spin.Spinner
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
		Streamer:   wow.New(args.stderr, spin.Get(spin.Monkey), ""),
		StreamSpinners: map[LogStreamLevel]spin.Spinner{
			LogStreamSuccess: {Frames: []string{viper.GetString("cli.success_emoji")}},
			LogStreamOk:      {Frames: []string{viper.GetString("cli.ok_emoji")}},
			LogStreamError:   {Frames: []string{viper.GetString("cli.error_emoji")}},
			LogStreamWarning: {Frames: []string{viper.GetString("cli.warning_emoji")}},
			LogStreamInfo:    {Frames: []string{viper.GetString("cli.info_emoji")}},
		},
	}
}

func (l *Logger) Success(message string) {
	println(l.stdout, aec.GreenF, message, aec.DefaultF)
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

func (l *Logger) Errorf(err error, format string, a ...interface{}) {
	_, _ = fmt.Fprintln(l.stderr, aec.LightRedF,
		fmt.Sprintf("%s: %v", fmt.Sprintf(format, a...), err), aec.DefaultF,
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

func (l *Logger) println(writer io.Writer, a ...interface{}) {
	var s = make([]string, len(a))
	for i := range a {
		s[i] = a[i].(string)
	}

	_, _ = fmt.Fprintln(l.stdout, strings.Join(s, ""))
}

func init() {
	log.SetOutput(ioutil.Discard)
}

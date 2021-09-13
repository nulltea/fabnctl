package term

import "fmt"

type LogStreamLevel int

const (
	LogStreamSuccess LogStreamLevel = iota
	LogStreamOk
	LogStreamError
	LogStreamWarning
	LogStreamInfo
)

// Stream wraps `fn` call into interactive logging with progress,
// displaying `start` message on loading, `complete` on successful end,
// and err return value on failure.
func (l *Logger) Stream(fn func() error, start, complete string) error {
	l.Streamer.Start()
	defer l.Streamer.Stop()

	l.Streamer.Text(start)
	if err := fn(); err != nil {
		l.Streamer.PersistWith(l.StreamSpinners[LogStreamError], " "+err.Error())
		return err
	}

	l.Streamer.PersistWith(l.StreamSpinners[LogStreamSuccess], " "+complete)

	return nil
}

// StreamLevel wraps `fn` call into interactive logging with loading,
// displaying `start` message on loading and custom persist on end.
func (l *Logger) StreamLevel(fn func() (level LogStreamLevel, msg string), start string) {
	l.Streamer.Start()
	defer l.Streamer.Stop()

	l.Streamer.Text(start)
	level, msg := fn()
	l.Streamer.PersistWith(l.StreamSpinners[level], " "+msg)
}

func (l *Logger) StreamTextf(format string, a ...interface{}) {
	l.Streamer.Text(fmt.Sprintf(format, a...))
}

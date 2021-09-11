package term

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
	l.streamer.Start()
	defer l.streamer.Stop()

	l.streamer.Text(start)
	if err := fn(); err != nil {
		l.streamer.PersistWith(l.streamSpinners[LogStreamError], " "+err.Error())
		return err
	}

	l.streamer.PersistWith(l.streamSpinners[LogStreamSuccess], " "+complete)

	return nil
}

// StreamLevel wraps `fn` call into interactive logging with loading,
// displaying `start` message on loading and custom persist on end.
func (l *Logger) StreamLevel(fn func() (level LogStreamLevel, msg string), start string) {
	l.streamer.Start()
	defer l.streamer.Stop()

	l.streamer.Text(start)
	level, msg := fn()
	l.streamer.PersistWith(l.streamSpinners[level], " "+msg)
}

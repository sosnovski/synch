package sql

// Logger is an interface that defines a method for logging warnings.
type Logger interface {
	// Warn logs a warning message with the given error.
	Warn(err error)
}

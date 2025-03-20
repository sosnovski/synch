package redis

// Option is a type representing an option function that can be used to modify a Driver instance.
type Option func(*Driver) error

// WithLogger is an option function that sets the logger used for logging warnings.
// If the provided logger is nil, it returns an ErrLoggerIsNil error.
func WithLogger(log Logger) Option {
	return func(d *Driver) error {
		if log == nil {
			return ErrLoggerIsNil
		}

		d.log = log

		return nil
	}
}

// WithPrefixLockKey is an option function that sets the redis prefix lock key.
// If the prefix is empty string, it returns an ErrPrefixKeyIsEmpty error.
func WithPrefixLockKey(prefix string) Option {
	return func(d *Driver) error {
		if prefix == "" {
			return ErrPrefixLockKeyIsEmpty
		}

		d.prefixLockKey = prefix

		return nil
	}
}

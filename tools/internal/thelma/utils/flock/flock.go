package flock

import (
	"context"
	"fmt"
	"github.com/gofrs/flock"
	"github.com/rs/zerolog/log"
	"time"
)

// flock is a thin wrapper around the open-source flock library with a simpler interface

// Options holds necessary attributes for a file lock with a timeout.
type Options struct {
	Path          string        // Path to file to use for lock
	RetryInterval time.Duration // RetryInterval how often to retry lock attempts
	Timeout       time.Duration // Timeout how long to wait for the lock before giving up
}

// Error for errors generated in the flock package
type Error struct {
	message string // message custom message for this error
	Err     error  // Err underlying error (if there was one)
}

// Error() implements error interface
func (err *Error) Error() string {
	return err.message
}

// newError constructs an Error from underlying error, format string and args
func newError(err error, format string, args ...interface{}) *Error {
	return &Error{
		message: fmt.Sprintf(format, args...),
		Err:     err,
	}
}

// WithLock executes a callback function with a global exclusive file-system lock
// If the lock is never acquired, the returned error will be of type flock.Error
// Else, the error will be whatever was returned by the callback function
func WithLock(options Options, syncFn func() error) error {
	lock := flock.New(options.Path)

	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()

	// Wait for lock
	log.Debug().Msgf("Attempting to acquire lock on %s, will time out after %s", options.Path, options.Timeout)
	locked, err := lock.TryLockContext(ctx, options.RetryInterval)
	if err != nil || !locked {
		return newError(err, "error acquiring lock on %s (timeout %s): %v", options.Path, options.Timeout, err)
	}
	log.Debug().Msgf("Acquired lock on %s", options.Path)

	// Defer unlock, logging an error if something goes wrong when we release the lock
	defer func() {
		if err := lock.Unlock(); err != nil {
			log.Error().Msgf("error releasing lock on %s: %v", options.Path, err)
		}
	}()

	// Invoke callback
	return syncFn()
}

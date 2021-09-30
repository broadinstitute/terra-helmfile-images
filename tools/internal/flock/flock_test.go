package flock

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"path"
	"regexp"
	"testing"
	"time"
)

// Base interval for all sleeps/polling etc in this test.
const baseInterval = 10 * time.Microsecond

const lockRetryInterval = baseInterval
const lockTimeout = 10_000 * lockRetryInterval

// This is the best I could do
func TestWithLockEnsuresConcurrentExecution(t *testing.T) {
	numRoutines := 25

	// should be small, we want every routine to get the lock in serial
	// _without_ triggering a timeout
	lockSleepTime := lockTimeout / 1000

	type result struct {
		err error
		id  int
	}

	ch := make(chan result)
	opts := testOptions(t)
	lockOwner := -1

	for i := 0; i < numRoutines; i++ {
		id := i // Copy to local variable to prevent leaks
		go func() {
			err := WithLock(opts, func() error {
				log.Info().Msgf("[%d] got lock", id)
				if lockOwner != -1 {
					return fmt.Errorf("[%d] another routine also owns the lock? %d", id, lockOwner)
				}
				lockOwner = id

				log.Info().Msgf("[%d] sleeping for %s", id, lockSleepTime)
				time.Sleep(lockSleepTime)

				log.Info().Msgf("[%d] woke, releasing lock", id)
				lockOwner = -1

				return nil
			})
			ch <- result{err, id}
		}()
	}

	// Verify none of the routines returned an error
	for i := 0; i < numRoutines; i++ {
		r := <-ch
		if r.err != nil {
			t.Errorf("[%d] unexpected error: %v", r.id, r.err)
		}
	}
}

func TestWithLockTimesOut(t *testing.T) {
	lockSleepTime := 10 * lockTimeout // we _want_ to trigger a timeout
	errCh := make(chan error)
	lockedCh := make(chan bool)
	opts := testOptions(t)

	// Launch a goroutine to claim lock in background
	go func() {
		errCh <- WithLock(opts, func() error {
			lockedCh <- true
			time.Sleep(lockSleepTime)
			return nil
		})
	}()

	// In foreground, try to get a lock and trigger a timeout
	<-lockedCh // Block until background routine has lock
	err := WithLock(opts, func() error {
		panic("I should never have obtained the lock!")
	})

	// Verify we got a timeout error and not something else
	assert.NotNil(t, err)
	flockErr, ok := err.(*Error)
	assert.True(t, ok)
	assert.Regexp(t, regexp.MustCompile("deadline exceeded"), flockErr.Error())

	// Verify the background routine didn't encounter an unexpected error
	assert.Nil(t, <-errCh, "Background routine should never return an error")
}

func TestWithLockReturnsCallbackError(t *testing.T) {
	err := WithLock(testOptions(t), func() error {
		return fmt.Errorf("fake error from callback")
	})
	assert.Equal(t, "fake error from callback", err.Error())
}

func testOptions(t *testing.T) Options {
	return Options{
		Path:          path.Join(t.TempDir(), "lock"),
		RetryInterval: lockRetryInterval,
		Timeout:       lockTimeout,
	}
}

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

// Base interval for all sleeps in this test.
// Unfortunately these concurrency tests depend on timing, and on slow infrastructure,
// like public GitHub actions runners, long intervals are needed to prevent unexpected
// failures.
const baseInterval = 1 * time.Millisecond

const lockRetryInterval = baseInterval
const lockTimeout = 1000 * baseInterval

func TestWithLockEnsuresConcurrentExecution(t *testing.T) {
	numWorkers := 25

	// should be small, we want every worker to get the lock in serial
	// _without_ triggering a timeout
	lockSleepTime := lockTimeout / 1000

	type result struct {
		err error
		id  int
	}

	ch := make(chan result)
	opts := testOptions(t)
	lockOwner := -1

	for i := 0; i < numWorkers; i++ {
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
	for i := 0; i < numWorkers; i++ {
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
			log.Info().Msgf("[lock thief] obtained the lock")
			lockedCh <- true
			log.Info().Msgf("[lock thief] sleeping for %s", lockSleepTime)
			time.Sleep(lockSleepTime)
			return nil
		})
	}()

	// In foreground, try to get a lock and trigger a timeout
	<- lockedCh // Block until background routine has lock
	log.Info().Msgf("[foreground] received a signal from thief that it has obtained the lock")
	start := time.Now()
	log.Info().Msgf("[foreground] calling withLock...")

	err := WithLock(opts, func() error {
		t.Fatalf("[foreground] I should never have obtained the lock!")
		return nil
	})

	actualEndTime := time.Now()
	actualWaitTime := actualEndTime.Sub(start)
	expectedEndTime := start.Add(lockTimeout)

	log.Info().Msgf("[foreground] WithLock returned %v after %s", err, actualWaitTime)

	// Allow a delta of 1/8th the existing timeout.
	// (we shift to avoid casting for a division)
	// okay - duration is a 64bit unsigned integer.
	delta := lockTimeout >> 3

	assert.WithinDuration(t, expectedEndTime, actualEndTime, delta, "Expected to get a timeout after about ~%s, got one after %s (allowed delta %s)", lockTimeout, actualWaitTime, delta)

	// Verify we got a timeout error and not something else
	assert.ErrorIs(t, err, err.(*Error), "Expected an Flock timeout error")
	assert.Regexp(t, regexp.MustCompile("deadline exceeded"), err.Error())

	// Verify the background routine didn't encounter an unexpected error
	assert.Nil(t, <-errCh, "Background routine should never return an error")
}

func TestWithLockReturnsCallbackError(t *testing.T) {
	err := WithLock(testOptions(t), func() error {
		return fmt.Errorf("fake error from callback")
	})
	assert.Error(t, err, "Expected error to propagate up from WithLock")
	assert.Equal(t, "fake error from callback", err.Error())
}

func testOptions(t *testing.T) Options {
	return Options{
		Path:          path.Join(t.TempDir(), "lock"),
		RetryInterval: lockRetryInterval,
		Timeout:       lockTimeout,
	}
}

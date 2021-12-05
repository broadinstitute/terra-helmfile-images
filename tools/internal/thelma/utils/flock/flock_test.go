package flock

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"path"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWithLockEnsuresConcurrentExecution(t *testing.T) {
	numWorkers := 25 // Number of worker routines that should attempt to grab the lock

	lockRetryInterval := 10 * time.Millisecond // How frequently flock should retry to get the lock
	lockSleepTime := 100 * time.Millisecond    // How long each worker should sleep after obtaining lock
	lockTimeout := 10 * time.Second            // How long each worker should wait for lock before returning timeout (shouldn't happen in this test)
	testTimeout := 2 * lockTimeout             // (20s) How long to wait for workers to complete before failing the test (shouldn't happen)

	type result struct {
		err error
		id  int
	}

	var wg sync.WaitGroup
	resultCh := make(chan result, numWorkers)
	opts := testOptions(t, lockRetryInterval, lockTimeout)

	var lockOwner int32 = -1

	for i := 0; i < numWorkers; i++ {
		id := i // Copy to local variable to prevent leaks
		wg.Add(1)
		go func() {
			err := WithLock(opts, func() error {
				log.Debug().Msgf("[%d] got lock", id)
				owner := atomic.LoadInt32(&lockOwner)
				if owner != -1 {
					return fmt.Errorf("[%d] another routine also owns the lock? %d", id, owner)
				}
				atomic.StoreInt32(&lockOwner, int32(id))

				log.Debug().Msgf("[%d] sleeping for %s", id, lockSleepTime)
				time.Sleep(lockSleepTime)

				log.Debug().Msgf("[%d] woke, releasing lock", id)
				atomic.StoreInt32(&lockOwner, -1)

				return nil
			})
			resultCh <- result{err, id}
			wg.Done()
		}()
	}

	// Verify results, but wrapped in a timeout, so that if something goes wrong
	// in this test we don't hang the whole suite
	testFinished := make(chan struct{})
	go func() {
		defer close(testFinished)
		log.Debug().Msg("Waiting for workers to finish")

		// Result results off the channel as they come in
		for i := 0; i < numWorkers; i++ {
			r := <-resultCh
			log.Debug().Msgf("Processing result: %v", r)
			if r.err != nil {
				t.Errorf("Unexpected error for worker %v: %v", r.id, r.err)
			}
		}

		// This should finish immediately, since all results have already been processed
		wg.Wait()
	}()

	select {
	case <-testFinished:
		log.Debug().Msg("Workers finished")
	case <-time.After(testTimeout):
		t.Fatalf("Test timed out after %s", testTimeout)
	}
}

func TestWithLockTimesOut(t *testing.T) {
	lockRetryInterval := 1 * time.Millisecond // How frequently flock should retry to get the lock
	lockTimeout := 2 * time.Second            // How long workers should wait for lock before returning timeout (we _want_ this to happen in this test)
	lockSleepTime := 2 * lockTimeout          // (4s) How long workers should sleep after obtaining lock (we _want_ to trigger a timeout)
	testTimeout := 10 * lockSleepTime         // (40s) How long to wait for workers to complete before failing the test (shouldn't happen)

	lockOpts := testOptions(t, lockRetryInterval, lockTimeout)

	type victimResult struct {
		err       error
		startTime time.Time
		stopTime  time.Time
	}

	var wg sync.WaitGroup
	thiefHasLockCh := make(chan struct{})
	thiefErrCh := make(chan error, 1)
	victimResultCh := make(chan victimResult, 1)

	// Launch a worker (the thief) to steal lock in background
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(thiefErrCh)

		thiefErrCh <- WithLock(lockOpts, func() error {
			log.Debug().Msgf("[thief] obtained the lock, sending signal")
			close(thiefHasLockCh)
			log.Debug().Msgf("[thief] sleeping for %s", lockSleepTime)
			time.Sleep(lockSleepTime)
			return nil
		})

		log.Debug().Msg("[thief] done")
	}()

	// Launch a second worker (the victim) to try to claim the lock. We _want_ this one to time out.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(victimResultCh)

		log.Debug().Msgf("[victim] waiting for thief to steal the lock")
		<-thiefHasLockCh
		log.Debug().Msgf("[victim] thief has stolen lock, calling withLock...")

		startTime := time.Now()
		err := WithLock(lockOpts, func() error {
			// this should never be called because we should hit a timeout
			t.Error("[victim] I should never have obtained the lock!")
			return nil
		})
		stopTime := time.Now()

		log.Debug().Msgf("[victim] WithLock returned %v after %s", err, stopTime.Sub(startTime))
		victimResultCh <- victimResult{err: err, startTime: startTime, stopTime: stopTime}
		log.Debug().Msg("[victim] done")
	}()

	// Verify results, but wrapped in a timeout, so that if
	// something goes wrong in this test we don't hang the whole suite
	testFinished := make(chan struct{})
	go func() {
		defer close(testFinished)

		// Verify results
		r := <-victimResultCh
		expectedStopTime := r.startTime.Add(lockTimeout)
		actualStopTime := r.stopTime
		actualWaitDuration := r.stopTime.Sub(r.startTime)

		// Allow a delta of 1/8th the existing timeout.
		// (we shift instead of dividing because integer division requires casting)
		delta := lockTimeout >> 3

		// Verify we timed out within the expected window
		assert.WithinDuration(t, expectedStopTime, actualStopTime, delta, "Expected to get a timeout after about ~%s, got one after %s (allowed delta %s)", lockTimeout, actualWaitDuration, delta)

		// Verify we got an flock timeout error and not something else
		assert.ErrorIs(t, r.err, r.err.(*Error), "Expected an Flock timeout error")
		assert.Regexp(t, regexp.MustCompile("deadline exceeded"), r.err.Error())

		// Verify the thief worker didn't encounter an unexpected error
		thiefErr := <-thiefErrCh
		assert.Nil(t, thiefErr, "Thief worker should never return an error, but got: %v", thiefErr)

		// Should return immediately, since by now results from all workers have been processed
		wg.Wait()
	}()

	select {
	case <-testFinished:
		log.Debug().Msg("Workers finished")
	case <-time.After(testTimeout):
		t.Fatalf("Timed out after %s waiting for workers to finish!", testTimeout)
	}
}

func TestWithLockReturnsCallbackError(t *testing.T) {
	lockRetryInterval := 1 * time.Millisecond
	lockTimeout := 1_000 * time.Millisecond

	// We don't expect any timeouts here! Just want to make sure errors are correctly propagated to caller
	err := WithLock(testOptions(t, lockRetryInterval, lockTimeout), func() error {
		return fmt.Errorf("fake error from callback")
	})
	assert.Error(t, err, "Expected error to propagate up from WithLock")
	assert.Equal(t, "fake error from callback", err.Error())
}

func testOptions(t *testing.T, lockRetryInterval time.Duration, lockTimeout time.Duration) Options {
	return Options{
		Path:          path.Join(t.TempDir(), "lock"),
		RetryInterval: lockRetryInterval,
		Timeout:       lockTimeout,
	}
}

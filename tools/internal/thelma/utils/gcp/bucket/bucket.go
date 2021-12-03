package bucket

import (
	"cloud.google.com/go/storage"
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/googleapi"
	"io"
	"net/http"
	"os"
	"time"
)

// Bucket offers higher-level operations on GCS buckets
type Bucket interface {
	// Name returns the name of the bucket
	Name() string

	// Close closes gcs client associated with this bucket
	Close() error

	// WaitForLock waits for a lock, timing out after maxTime. It returns a lock id / object generation number
	// that must be passed in to ReleaseLock
	WaitForLock(objectPath string, maxWait time.Duration) (int64, error)

	// DeleteStaleLock deletes a stale lock file if it exists and is older than staleAge
	DeleteStaleLock(objectPath string, staleAge time.Duration) error

	// ReleaseLock removes a lockfile
	ReleaseLock(objectPath string, generation int64) error

	// Exists returns true if the object exists, false otherwise
	Exists(objectPath string) (bool, error)

	// Upload uploads a local file to the bucket
	Upload(localPath string, objectPath string, cacheControl string) error

	// Download downloads an object in the bucket to a local file
	Download(objectPath string, localPath string) error
}

// Real implementation of Implements Bucket
type bucket struct {
	name   string
	ctx    context.Context
	client *storage.Client
}

// NewBucket creates a new Bucket
func NewBucket(name string) (*bucket, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	return &bucket{
		name:   name,
		ctx:    context.Background(),
		client: client,
	}, nil
}

// Close closes gcs client associated with this bucket
func (b *bucket) Close() error {
	return b.client.Close()
}

// Name returns the name of this bucket
func (b *bucket) Name() string {
	return b.name
}

// WaitForLock waits for a lock, timing out after maxTime. It returns a lock id / object generation number
// that must be passed in to ReleaseLock
func (b *bucket) WaitForLock(objectPath string, maxWait time.Duration) (int64, error) {
	obj := b.getObject(objectPath)
	obj = obj.If(storage.Conditions{DoesNotExist: true})

	ctx, cancelFn := context.WithTimeout(b.ctx, maxWait)
	defer cancelFn()

	backoff := 10 * time.Millisecond
	attempt := 1

	for {
		log.Debug().Msgf("Attempt %d to obtain lock gs://%s/%s", attempt, b.name, objectPath)

		writer := obj.NewWriter(ctx)
		_, writeErr := writer.Write([]byte(""))
		closeErr := writer.Close()

		if writeErr == nil && closeErr == nil {
			// Success!
			generation := writer.Attrs().Generation
			log.Debug().Msgf("Successfully obtained lock gs://%s/%s on attempt %d (generation: %d)", b.name, objectPath, attempt, generation)
			return generation, nil
		}

		// We failed to grab the lock. Either someone else has it or something went wrong. Either way, retry after backoff
		if writeErr != nil {
			log.Warn().Msgf("Unexpected error attempting to write to lock file gs://%s/%s: %v", b.name, objectPath, writeErr)
		}
		if closeErr != nil {
			if isPreconditionFailed(closeErr) {
				log.Debug().Msgf("Another process has a lock on gs://%s/%s, will sleep %s and retry", b.name, objectPath, backoff)
			} else {
				log.Warn().Msgf("Unexpected error attempting to close lock file gs://%s/%s: %v", b.name, objectPath, closeErr)
			}
		}

		select {
		case <-time.After(backoff):
			backoff *= 2
			attempt++
			continue
		case <-ctx.Done():
			return 0, fmt.Errorf("timed out after %s waiting for lock gs://%s/%s: %v", maxWait, b.name, objectPath, ctx.Err())
		}
	}
}

// DeleteStaleLock deletes a stale lock file if it exists and is older than staleAge
func (b *bucket) DeleteStaleLock(objectPath string, staleAge time.Duration) error {
	obj := b.getObject(objectPath)
	attrs, err := obj.Attrs(b.ctx)
	if err == storage.ErrObjectNotExist {
		log.Debug().Msgf("No lock file found: gs://%s/%s", b.name, objectPath)
		return nil
	}
	if err != nil {
		return fmt.Errorf("error loading attributes for lock object gs://%s/%s: %v", b.name, objectPath, err)
	}

	lockAge := time.Since(attrs.Created)
	if lockAge < staleAge {
		// lock file exists but is not stale
		log.Debug().Msgf("Lock file gs://%s/%s is not stale, won't delete it (creation time: %s, age: %s, max age: %s)", b.name, objectPath, attrs.Created, lockAge, staleAge)
		return nil
	}

	log.Warn().Msgf("Deleting stale lock file gs://%s/%s (creation time: %s, age: %s, max age: %s)", b.name, objectPath, attrs.Created, lockAge, staleAge)

	// Use a generation precondition to make sure we don't run into a race condition with another process
	condObj := obj.If(storage.Conditions{GenerationMatch: attrs.Generation})

	if err := condObj.Delete(b.ctx); err != nil {
		if isPreconditionFailed(err) {
			log.Warn().Msgf("Another process deleted stale lock gs://%s/%s before we could", b.name, objectPath)
			return nil
		}

		return fmt.Errorf("error deleting stale lock file gs://%s/%s: %v", b.name, objectPath, err)
	}

	return nil
}

// ReleaseLock removes a lockfile
func (b *bucket) ReleaseLock(objectPath string, generation int64) error {
	obj := b.getObject(objectPath)

	obj = obj.If(storage.Conditions{GenerationMatch: generation})
	if err := obj.Delete(b.ctx); err != nil {
		if isPreconditionFailed(err) {
			log.Warn().Msgf("Attempted to delete lock gs://%s/%s, but another process had already claimed it", b.name, objectPath)
			return nil
		}
		return fmt.Errorf("error deleting lock file gs://%s/%s: %v", b.name, objectPath, err)
	}

	log.Debug().Msgf("Successfully released lock gs://%s/%s (generation %v)", b.name, objectPath, generation)
	return nil
}

// Delete deletes an object in the bucket
func (b *bucket) Delete(objectPath string) error {
	object := b.getObject(objectPath)

	if err := object.Delete(b.ctx); err != nil {
		return fmt.Errorf("error deleting gs://%s/%s: %v", b.name, objectPath, err)
	}

	return nil
}

// Exists returns true if the object exists, false otherwise
func (b *bucket) Exists(objectPath string) (bool, error) {
	object := b.getObject(objectPath)
	_, err := object.Attrs(b.ctx)
	if err == nil {
		return true, nil
	}
	if err == storage.ErrObjectNotExist {
		return false, nil
	}
	return false, err
}

// Upload uploads a local file to the bucket
func (b *bucket) Upload(localPath string, objectPath string, cacheControl string) error {
	errPrefix := fmt.Sprintf("error uploading file:///%s to gs://%s/%s", localPath, b.name, objectPath)

	obj := b.getObject(objectPath)

	fileReader, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("%s: failed to open file: %v", errPrefix, err)
	}

	objWriter := obj.NewWriter(b.ctx)

	objWriter.CacheControl = cacheControl

	if _, err := io.Copy(objWriter, fileReader); err != nil {
		return fmt.Errorf("%s: write failed: %v", errPrefix, err)
	}
	if err := objWriter.Close(); err != nil {
		return fmt.Errorf("%s: error closing object writer: %v", errPrefix, err)
	}
	if err := fileReader.Close(); err != nil {
		return fmt.Errorf("%s: error closing local reader: %v", errPrefix, err)
	}

	log.Debug().Msgf("Uploaded %s to gs://%s/%s", localPath, b.Name(), objectPath)

	return nil
}

// Download downloads an object in the bucket to a local file
func (b *bucket) Download(objectPath string, localPath string) error {
	errPrefix := fmt.Sprintf("error downloading gs://%s/%s to file:///%s", b.name, objectPath, localPath)
	obj := b.getObject(objectPath)

	fileWriter, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("%s: failed to open file: %v", errPrefix, err)
	}

	objReader, err := obj.NewReader(b.ctx)
	if err != nil {
		return fmt.Errorf("%s: failed to create object reader: %v", errPrefix, err)
	}
	if _, err := io.Copy(fileWriter, objReader); err != nil {
		return fmt.Errorf("%s: copy failed: %v", errPrefix, err)
	}
	if err := objReader.Close(); err != nil {
		return fmt.Errorf("%s: error closing object reader: %v", errPrefix, err)
	}
	if err := fileWriter.Close(); err != nil {
		return fmt.Errorf("%s: error closing local writer: %v", errPrefix, err)
	}

	log.Debug().Msgf("Downloaded gs://%s/%s to %s", b.Name(), objectPath, localPath)

	return nil
}

func (b *bucket) getObject(objectPath string) *storage.ObjectHandle {
	return b.client.Bucket(b.name).Object(objectPath)
}

func isPreconditionFailed(err error) bool {
	if err == nil {
		return false
	}
	if googleErr, ok := err.(*googleapi.Error); ok {
		if googleErr.Code == http.StatusPreconditionFailed {
			return true
		}
	}
	return false
}

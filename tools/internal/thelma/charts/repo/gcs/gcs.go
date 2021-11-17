package gcs

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

type Bucket struct {
	bucketName string
	ctx context.Context
}

func New(bucketName string) *Bucket {
	return &Bucket{
		bucketName: bucketName,
		ctx: context.Background(),
	}
}

func (bucket *Bucket) Name() string {
	return bucket.bucketName
}

func (bucket *Bucket) WaitForLock(objectPath string, description string, maxWait time.Duration) (generation int64, err error) {
	err = bucket.withObject(objectPath, func(obj *storage.ObjectHandle) error {
		obj = obj.If(storage.Conditions{DoesNotExist: true})

		ctx, cancelFn := context.WithTimeout(bucket.ctx, maxWait)
		defer cancelFn()

		backoff := 10 * time.Millisecond
		attempt := 1

		for {
			log.Debug().Msgf("Attempt %d to obtain lock gs://%s/%s", attempt, bucket.bucketName, objectPath)

			writer := obj.NewWriter(ctx)
			_, writeErr := writer.Write([]byte(description))
			closeErr := writer.Close()

			if writeErr == nil && closeErr == nil {
				// Success!
				generation = writer.Attrs().Generation
				log.Debug().Msgf("Successfully obtained lock gs://%s/%s on attempt %d (generation: %d)", bucket.bucketName, objectPath, attempt, generation)
				return nil
			}

			// We failed to grab the lock. Either someone else has it or something went wrong. Either way, retry after backoff
			if writeErr != nil {
				log.Warn().Msgf("Unexpected error attempting to write to lock file gs://%s/%s: %v", bucket.bucketName, objectPath, writeErr)
			}
			if closeErr != nil {
				if isPreconditionFailed(closeErr) {
					log.Debug().Msgf("Another process has a lock on gs://%s/%s, will sleep %s and retry", bucket.bucketName, objectPath, backoff)
				} else {
					log.Warn().Msgf("Unexpected error attempting to close lock file gs://%s/%s: %v", bucket.bucketName, objectPath, closeErr)
				}
			}

			select {
			case <-time.After(backoff):
				backoff *= 2
				attempt++
				continue
			case <-ctx.Done():
				return fmt.Errorf("timed out after %s waiting for lock gs://%s/%s: %v", maxWait, bucket.bucketName, objectPath, ctx.Err())
			}
		}
	})
	return generation, err
}

func (bucket *Bucket) DeleteStaleLockIfExists(objectPath string, staleAge time.Duration) error {
	return bucket.withObject(objectPath, func(obj *storage.ObjectHandle) error {
		attrs, err := obj.Attrs(bucket.ctx)
		if err == storage.ErrObjectNotExist {
			// no stale lock to delete
			log.Debug().Msgf("No lock file found: gs://%s/%s", bucket.bucketName, objectPath)
			return nil
		}
		if err != nil {
			// unexpected error loading attributes
			return fmt.Errorf("error loading attributes for lock object gs://%s/%s: %v", bucket.bucketName, objectPath, err)
		}

		lockAge := time.Now().Sub(attrs.Created)
		if lockAge < staleAge {
			// lock file exists but is not stale
			log.Debug().Msgf("Lock file gs://%s/%s is %s old, max age is %s (creation time: %s), won't delete it", bucket.bucketName, objectPath, lockAge, staleAge, attrs.Created)
			return nil
		}

		log.Warn().Msgf("Lock file gs://%s/%s is %s old, max age is %s (creation time: %s), deleting it", bucket.bucketName, objectPath, lockAge, staleAge, attrs.Created)

		// Use a generation precondition to make sure we don't run into a race condition with another process
		// deleting the real lock file
		condObj := obj.If(storage.Conditions{GenerationMatch: attrs.Generation})

		if err := condObj.Delete(bucket.ctx); err != nil {
			if isPreconditionFailed(err) {
				log.Warn().Msgf("Another process deleted stale lock gs://%s/%s before we could", bucket.bucketName, objectPath)
				return nil
			}

			return fmt.Errorf("error deleting stale lock file gs://%s/%s: %v", bucket.bucketName, objectPath, err)
		}

		return nil
	})
}

func (bucket *Bucket) ReleaseLock(objectPath string, generation int64) error {
	return bucket.withObject(objectPath, func(obj *storage.ObjectHandle) error {
		obj = obj.If(storage.Conditions{GenerationMatch: generation})
		if err := obj.Delete(bucket.ctx); err != nil {
			if isPreconditionFailed(err) {
				log.Warn().Msgf("Attempted to delete lock gs://%s/%s, but another process had already claimed it", bucket.bucketName, objectPath)
				return nil
			}
			return fmt.Errorf("error deleting lock file gs://%s/%s: %v", bucket.bucketName, objectPath, err)
		}
		log.Debug().Msgf("Successfully released lock gs://%s/%s (generation %v)", bucket.bucketName, objectPath, generation)
		return nil
	})
}

func (bucket *Bucket) Delete(objectPath string) error {
	return bucket.withObject(objectPath, func(object *storage.ObjectHandle) error {
		if err :=  object.Delete(bucket.ctx); err != nil {
			return fmt.Errorf("error deleting gs://%s/%s: %v", bucket.bucketName, objectPath, err)
		}

		return nil
	})
}

func (bucket *Bucket) Exists(objectPath string) (exists bool, returnErr error) {
	_ = bucket.withObject(objectPath, func(object *storage.ObjectHandle) error {
		_, err := object.Attrs(bucket.ctx)
		if err == nil {
			exists = true
		} else if err == storage.ErrObjectNotExist {
			exists = false
		} else {
			returnErr = err
		}
		return nil
	})
	return
}

func (bucket *Bucket) Upload(localPath string, objectPath string, cacheControl string) error {
	errPrefix := fmt.Sprintf("error uploading file:///%s to gs://%s/%s", localPath, bucket.bucketName, objectPath)

	return bucket.withObject(objectPath, func(obj *storage.ObjectHandle) error {
		fileReader, err := os.Open(localPath)
		if err != nil {
			return fmt.Errorf("%s: failed to open file: %v", errPrefix, err)
		}

		objWriter := obj.NewWriter(bucket.ctx)

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

		return nil
	})
}

func (bucket *Bucket) Download(objectPath string, localPath string) error {
	errPrefix := fmt.Sprintf("error downloading gs://%s/%s to file:///%s", bucket.bucketName, objectPath, localPath)
	return bucket.withObject(objectPath, func(obj *storage.ObjectHandle) error {
		fileWriter, err := os.Create(localPath)
		if err != nil {
			return fmt.Errorf("%s: failed to open file: %v", errPrefix, err)
		}

		objReader, err := obj.NewReader(bucket.ctx)
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

		return nil
	})
}

func (bucket *Bucket) withObject(objectPath string, fn func (object *storage.ObjectHandle) error) error {
	client, err := storage.NewClient(bucket.ctx)
	if err != nil {
		return fmt.Errorf("error creating GCS client: %v", err)
	}

	bucketHandle := client.Bucket(bucket.bucketName)
	object := bucketHandle.Object(objectPath)
	userFnErr := fn(object)

	if err := client.Close(); err != nil {
		if userFnErr != nil {
			return fmt.Errorf("bucket error (%s): %v;\nerror closing client: %v", bucket.bucketName, userFnErr, err)
		}
		return err
	}

	return userFnErr
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

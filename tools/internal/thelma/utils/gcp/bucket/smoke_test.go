//go:build smoke
// +build smoke

package bucket

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"os"
	"path"
	"testing"
	"time"
)

type testHarness struct {
	bucket       *bucket
	objectPrefix string
}

// This file contains a smoke test that talks to a real GCS bucket. By default it is ignored by `go test`.
// You can run it with:
//   go test ./... -tags smoke

// Integration test for gcs package
const testBucket = "thelma-gcs-integration-test" // lives in dsp-tools-k8s project

const lockStaleTimeout = 5 * time.Second
const lockWaitTimeout = 1 * time.Second
const testCacheControlHeader = "public, max-age=1337"

func TestLockAndRelease(t *testing.T) {
	harness := newTestHarness(t)
	bucket := harness.bucket
	lockObj := harness.lockPath()

	assertObjectDoesNotExist(t, bucket, lockObj)

	err := bucket.DeleteStaleLock(lockObj, lockStaleTimeout)
	assert.NoError(t, err, "delete stale lock should not raise err if lock does not exist")

	generation, err := bucket.WaitForLock(lockObj, lockWaitTimeout)
	assert.NoError(t, err)

	assertObjectExists(t, bucket, lockObj)

	_, err = bucket.WaitForLock(lockObj, lockWaitTimeout)
	assert.Error(t, err)
	assert.Regexp(t, "timed out after 1s waiting for lock", err.Error())

	err = bucket.ReleaseLock(lockObj, generation)
	assert.NoError(t, err)

	assertObjectDoesNotExist(t, bucket, lockObj)
}

func TestDeleteStaleLock(t *testing.T) {
	harness := newTestHarness(t)
	bucket := harness.bucket
	lockObj := harness.lockPath()

	assertObjectDoesNotExist(t, bucket, lockObj)

	err := bucket.DeleteStaleLock(lockObj, lockStaleTimeout)
	assert.NoError(t, err, "delete should not raise error if lock no exist")

	_, err = bucket.WaitForLock(lockObj, lockWaitTimeout)
	assert.NoError(t, err)

	assertObjectExists(t, bucket, lockObj)

	err = bucket.DeleteStaleLock(lockObj, lockStaleTimeout)
	assert.NoError(t, err)

	assertObjectExists(t, bucket, lockObj)

	time.Sleep(lockStaleTimeout + lockStaleTimeout/10)

	err = bucket.DeleteStaleLock(lockObj, lockStaleTimeout)
	assert.NoError(t, err)

	assertObjectDoesNotExist(t, bucket, lockObj)
}

func TestUploadAndDownload(t *testing.T) {
	harness := newTestHarness(t)
	bucket := harness.bucket
	testContent := "foo\n"
	testObj := harness.testObjPath()
	testDir := t.TempDir()
	testFile1 := path.Join(testDir, "file1")
	testFile2 := path.Join(testDir, "file2")

	assertObjectDoesNotExist(t, bucket, testObj)

	err := os.WriteFile(testFile1, []byte(testContent), 0600)
	assert.NoError(t, err)

	err = bucket.Upload(testFile1, testObj, testCacheControlHeader)
	assert.NoError(t, err)

	assertObjectExists(t, bucket, testObj)

	err = bucket.Download(testObj, testFile2)
	assert.NoError(t, err)

	assert.FileExists(t, testFile2)
	content, err := os.ReadFile(testFile2)
	assert.NoError(t, err)
	assert.Equal(t, testContent, string(content))

	attrs, err := bucket.getObject(testObj).Attrs(bucket.ctx)
	assert.NoError(t, err)
	assert.Equal(t, testCacheControlHeader, attrs.CacheControl, "Upload should set cache control header")
}

func newTestHarness(t *testing.T) *testHarness {
	timestamp := time.Now().UTC().Format("20060102.150405")
	prefix := fmt.Sprintf("%s.%x", timestamp, rand.Int())

	return &testHarness{
		bucket:       setupBucket(t),
		objectPrefix: prefix,
	}
}

func (th *testHarness) lockPath() string {
	return path.Join(th.objectPrefix, "integration-test.lk")
}

func (th *testHarness) testObjPath() string {
	return path.Join(th.objectPrefix, "test.obj")
}

func setupBucket(t *testing.T) *bucket {
	bucket, err := NewBucket(testBucket)
	assert.NoError(t, err)
	t.Cleanup(func() {
		err := bucket.Close()
		if err != nil {
			t.Fatal(err)
		}
	})

	return bucket
}

func assertObjectExists(t *testing.T, bucket *bucket, objectPath string) {
	exists, err := bucket.Exists(objectPath)
	assert.NoError(t, err, "unexpected error checking existence of gs://%s/%s", bucket.Name(), objectPath)
	assert.True(t, exists, "%s should exist in bucket, but does not", objectPath)
}

func assertObjectDoesNotExist(t *testing.T, bucket *bucket, objectPath string) {
	exists, err := bucket.Exists(objectPath)
	assert.NoError(t, err, "unexpected error checking existence of gs://%s/%s", bucket.Name(), objectPath)
	assert.False(t, exists, "%s should not exist in bucket, but does", objectPath)
}

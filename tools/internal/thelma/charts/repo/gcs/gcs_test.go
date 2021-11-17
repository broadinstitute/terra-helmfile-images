package gcs

import (
	"fmt"
	"testing"
	"time"
)

func TestUploadAndDownload(t *testing.T) {
	cfg := New("terra-helm-thirdparty")

	lockPath := "choover-test-lock.txt"
//	err := cfg.UpdateRepo("/tmp/foo.txt", "choover-test-foo.txt")
	err := cfg.DeleteStaleLockIfExists(lockPath, 5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}

	generation, err := cfg.WaitForLock(lockPath, "my lock", 5 * time.Second)
	fmt.Printf("Generation: %v", generation)
	if err != nil {
		t.Fatal(err)
	}

	err = cfg.ReleaseLock(lockPath, generation)
	if err != nil {
		t.Fatal(err)
	}
}
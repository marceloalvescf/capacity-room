package worker

import (
	"errors"
	"testing"
)

func TestCheckSnapshotSize(t *testing.T) {
	if err := checkSnapshotSize(33554432); err != nil {
		t.Fatalf("32 MiB is allowed: %v", err)
	}
	if err := checkSnapshotSize(33554433); !errors.Is(err, errSnapshotTooLarge) {
		t.Fatalf("err = %v", err)
	}
}

func TestRetriedCodes(t *testing.T) {
	for code, want := range map[string]bool{
		"ENDPOINT_UNREACHABLE": true, "INTERNAL_FAILURE": true, "COLLECTION_TIMEOUT": false,
		"NOTHING_TO_REPORT": false, "SNAPSHOT_TOO_LARGE": false, "ENDPOINT_AUTH_REJECTED": false,
	} {
		if retried(code) != want {
			t.Errorf("retried(%s) = %v, want %v", code, !want, want)
		}
	}
}

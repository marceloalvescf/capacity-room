package worker

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

func TestClassifyValuesOutOfRange(t *testing.T) {
	err := fmt.Errorf("encode: %w", fmt.Errorf("%w: %w", snapshot.ErrNotFinite, errors.New("json: unsupported value: +Inf")))
	code, stop := classify(context.Background(), err)
	if code != "SOURCE_VALUES_OUT_OF_RANGE" || stop != nil {
		t.Fatalf("classify = %q, %v", code, stop)
	}
	if retried(code) {
		t.Fatal("SOURCE_VALUES_OUT_OF_RANGE is retried")
	}
}

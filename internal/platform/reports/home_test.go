//go:build postgres

package reports

import (
	"context"
	"testing"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/platform/profiles"
)

func TestHomeKeepsTheActiveOrder(t *testing.T) {
	f := newFixture(t)
	got, err := f.svc.Home(context.Background(), f.read, []profiles.Summary{
		{ID: "z", DisplayName: "Zed"}, {ID: "a", DisplayName: "Alpha"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ProfileID != "z" || got[0].DisplayName != "Zed" ||
		got[1].ProfileID != "a" || got[1].DisplayName != "Alpha" {
		t.Fatalf("clusters = %+v", got)
	}
	if got[0].Latest != nil || got[0].Newest != nil {
		t.Fatalf("profile without reports has sides: %+v", got[0])
	}
}

func TestHomeReadsTheClockOnce(t *testing.T) {
	f := newFixture(t)
	calls := 0
	f.svc.Now = func() time.Time { calls++; return fixedNow }
	f.seedReport(t, "collected", "complete", fixedNow.Add(time.Hour), "")
	if _, err := f.svc.Home(context.Background(), f.read, []profiles.Summary{{ID: "prod", DisplayName: "Production"}}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("clock read %d times", calls)
	}
}

func TestHomeMarksAnExpiredNewest(t *testing.T) {
	f := newFixture(t)
	id := f.seedReport(t, "collected", "complete", fixedNow.Add(-time.Minute), "")
	got, err := f.svc.Home(context.Background(), f.read, []profiles.Summary{{ID: "prod", DisplayName: "Production"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Latest != nil || got[0].Newest == nil ||
		got[0].Newest.ID != id || !got[0].Newest.Expired {
		t.Fatalf("clusters = %+v", got)
	}
}

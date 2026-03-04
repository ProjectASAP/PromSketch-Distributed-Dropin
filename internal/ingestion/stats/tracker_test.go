package stats

import (
	"testing"
	"time"
)

func TestTrackerSnapshot(t *testing.T) {
	tracker := NewTracker(20*time.Millisecond, 3)
	tracker.Start()
	defer tracker.Stop()

	tracker.AddSamples(10)
	time.Sleep(30 * time.Millisecond)

	s1 := tracker.Snapshot()
	if s1.TotalSamples != 10 {
		t.Fatalf("unexpected total samples: got=%d want=10", s1.TotalSamples)
	}
	if s1.IntervalSeconds <= 0 {
		t.Fatalf("expected positive interval seconds, got=%f", s1.IntervalSeconds)
	}

	tracker.AddSamples(20)
	time.Sleep(30 * time.Millisecond)

	s2 := tracker.Snapshot()
	if s2.TotalSamples != 30 {
		t.Fatalf("unexpected total samples: got=%d want=30", s2.TotalSamples)
	}
	if s2.RatePerSec <= 0 {
		t.Fatalf("expected positive rate, got=%f", s2.RatePerSec)
	}
	if s2.AvgRatePerSec <= 0 {
		t.Fatalf("expected positive avg rate, got=%f", s2.AvgRatePerSec)
	}
}

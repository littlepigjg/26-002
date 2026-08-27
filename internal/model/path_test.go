package model

import (
	"math"
	"testing"
	"time"
)

// TestAccumulateDuration_OverflowSaturates reproduces the reported bug: three
// large positive durations (math.MaxInt64/2 + 1 each) used to wrap to a large
// negative number (-4611686018427387905). With saturating addition the running
// total must clamp to math.MaxInt64 and can never go negative.
func TestAccumulateDuration_OverflowSaturates(t *testing.T) {
	big := int64(math.MaxInt64/2 + 1) // 4611686018427387904

	// Reset fault injector so production (saturating) logic is exercised.
	SetFaultInjector(nil)
	t.Cleanup(func() { SetFaultInjector(nil) })

	var total int64
	for i := 0; i < 3; i++ {
		total = AccumulateDuration(total, big)
		if total < 0 {
			t.Fatalf("step %d: total went negative: %d", i+1, total)
		}
	}

	if total != math.MaxInt64 {
		t.Fatalf("expected saturation at math.MaxInt64 (%d), got %d", int64(math.MaxInt64), total)
	}
}

// TestAccumulateDuration_OverflowSaturatesRaw reproduces the same wrap with a
// direct saturating MaxInt64 + positive delta that previously fell through the
// broken "fast path" (which only guarded the negative direction).
func TestAccumulateDuration_SaturatesFromMaxPlusPositive(t *testing.T) {
	SetFaultInjector(nil)
	t.Cleanup(func() { SetFaultInjector(nil) })

	got := AccumulateDuration(math.MaxInt64, 1)
	if got != math.MaxInt64 {
		t.Fatalf("MaxInt64 + 1 should saturate to MaxInt64, got %d", got)
	}
}

// TestAccumulateDuration_NegativeOverflowSaturates ensures symmetric negative
// saturation so the saturating logic is correct in both directions.
func TestAccumulateDuration_NegativeOverflowSaturates(t *testing.T) {
	SetFaultInjector(nil)
	t.Cleanup(func() { SetFaultInjector(nil) })

	// MinInt64 + a negative delta overflows downward.
	got := AccumulateDuration(math.MinInt64, -1)
	if got != math.MinInt64 {
		t.Fatalf("MinInt64 - 1 should saturate to MinInt64, got %d", got)
	}
}

// TestAccumulateDuration_NormalAddition confirms the non-overflow path is a
// plain sum, including the mixed-sign case where no clamp should occur.
func TestAccumulateDuration_NormalAddition(t *testing.T) {
	SetFaultInjector(nil)
	t.Cleanup(func() { SetFaultInjector(nil) })

	cases := []struct {
		total, delta, want int64
	}{
		{0, 100, 100},
		{50, 50, 100},
		{100, -30, 70}, // mixed signs: no clamp, just add
		{math.MaxInt64 - 10, 5, math.MaxInt64 - 5},
	}
	for _, c := range cases {
		if got := AccumulateDuration(c.total, c.delta); got != c.want {
			t.Errorf("AccumulateDuration(%d, %d) = %d; want %d", c.total, c.delta, got, c.want)
		}
	}
}

// TestPathSequence_ComputeDuration_NoNegativeOverflow is the end-to-end repro:
// a path with three large-duration nodes must report a non-negative
// TotalDuration instead of the previously observed -4611686018427387905.
func TestPathSequence_ComputeDuration_NoNegativeOverflow(t *testing.T) {
	big := int64(math.MaxInt64/2 + 1)

	ps := NewPathSequence("user-1", "ses-1")
	for i := 0; i < 3; i++ {
		ps.AppendNode(PathNode{
			PageURL:   "/page",
			Order:     i,
			Duration:  big,
			Timestamp: time.Unix(int64(i), 0),
		})
	}

	got := ps.ComputeDuration()
	if got < 0 {
		t.Fatalf("TotalDuration went negative: %d", got)
	}
	if ps.TotalDuration != math.MaxInt64 {
		t.Fatalf("TotalDuration = %d; want saturated math.MaxInt64 (%d)",
			ps.TotalDuration, int64(math.MaxInt64))
	}
}

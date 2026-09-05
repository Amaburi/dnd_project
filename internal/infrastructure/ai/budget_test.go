package ai

import (
	"testing"
	"time"
)

func at(minutes int) time.Time {
	return time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC).Add(time.Duration(minutes) * time.Minute)
}

// The budget exists because a free tier is a real limit and a runaway loop is
// a real possibility. Refusing locally is cheaper than being refused remotely,
// and far cheaper than being billed.
func TestBudgetRefusesPastTheLimit(t *testing.T) {
	now := at(0)
	b := newBudget(3, func() time.Time { return now })

	for i := 0; i < 3; i++ {
		if err := b.reserve(); err != nil {
			t.Fatalf("call %d was refused: %v", i+1, err)
		}
	}
	err := b.reserve()
	if err == nil {
		t.Fatal("the fourth call was allowed past a limit of three")
	}

	// The refusal has to say when it will lift, or a caller can only guess.
	aiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("error is %T, want *Error", err)
	}
	if aiErr.Retriable {
		t.Error("a budget refusal is not retriable: retrying immediately cannot help")
	}
}

// The window slides: an hour after the first call, that call stops counting.
func TestBudgetWindowSlides(t *testing.T) {
	now := at(0)
	b := newBudget(2, func() time.Time { return now })

	if err := b.reserve(); err != nil {
		t.Fatalf("first call: %v", err)
	}
	now = at(30)
	if err := b.reserve(); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if err := b.reserve(); err == nil {
		t.Fatal("a third call inside the hour was allowed")
	}

	// Sixty-one minutes after the first, it has aged out and one slot is free.
	now = at(61)
	if err := b.reserve(); err != nil {
		t.Errorf("a call after the window slid was refused: %v", err)
	}
	// But the second call, made at 30 minutes, still counts.
	if err := b.reserve(); err == nil {
		t.Error("the still-recent call stopped counting")
	}
}

// Zero disables it, the way every other limit in this project does.
func TestBudgetOfZeroIsDisabled(t *testing.T) {
	now := at(0)
	b := newBudget(0, func() time.Time { return now })

	for i := 0; i < 500; i++ {
		if err := b.reserve(); err != nil {
			t.Fatalf("an unlimited budget refused call %d: %v", i+1, err)
		}
	}
}

// A budget that grew one entry per call would be a slow leak in a long-lived
// process, so old entries are dropped rather than kept.
func TestBudgetDoesNotGrowWithoutBound(t *testing.T) {
	now := at(0)
	b := newBudget(5, func() time.Time { return now })

	for minute := 0; minute < 600; minute++ {
		now = at(minute)
		_ = b.reserve()
	}
	if len(b.calls) > 5 {
		t.Errorf("the budget is holding %d timestamps, want at most 5", len(b.calls))
	}
}

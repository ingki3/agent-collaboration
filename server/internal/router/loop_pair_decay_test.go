// FR-3.5 — v0.19.32 (Director 결정 2026-10-17): max_pair_roundtrips decays
// (halves, floor) on a third-party interjection instead of resetting to 0
// (loop_golden_test.go E4-04 pins the single-interjection case), plus a new,
// completely independent time-window counter (max_pair_roundtrips_window)
// that closes the mathematical gap decay alone leaves open — an interjection
// timed exactly right can hold the decaying counter in permanent oscillation
// below the limit, but it cannot make hops fall outside a fixed clock window.
package router

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestPairRoundtripsDecayHasAMathematicalGap demonstrates the PRD's own
// reasoning for why decay alone is not enough (v0.19.32): interjecting on
// EVERY cycle (not just occasionally) finds a fixed point where the decayed
// counter stops climbing at all — "감쇄의 수학적 한계(적대적 타이밍으로 상한
// 아래 영구 진동 가능)". This is not a bug in pairRoundtrips; it is exactly
// the gap pairRoundtripsWindow exists to close (see
// TestCheckLoopLimitsPairRoundtripsWindowTripsIndependentlyOfDecay below).
func TestPairRoundtripsDecayHasAMathematicalGap(t *testing.T) {
	t0 := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	var h []Hop
	at := t0
	step := func(d time.Duration) time.Time { at = at.Add(d); return at }
	for i := 0; i < 8; i++ {
		h = append(h,
			Hop{FromAgent: agLead, ToAgent: agR, At: step(time.Minute)},
			Hop{FromAgent: agR, ToAgent: agLead, At: step(time.Minute)},
			Hop{FromAgent: agW, ToAgent: agLead, At: step(time.Minute)}, // interjection, halves
		)
	}
	next := Hop{FromAgent: agLead, ToAgent: agR, At: step(time.Minute)}
	// +1, +1, /2 per cycle settles at a fixed point of 1 (3/2 floors back to
	// 1) however many cycles run — 8 full roundtrips' worth of activity and
	// the decaying counter never leaves single digits.
	if got := pairRoundtrips(h, next); got != 1 {
		t.Errorf("pairRoundtrips = %d, want the fixed point 1 — if this changes, re-check whether the oscillation the PRD describes still exists", got)
	}
}

func TestPairRoundtripsDecayRoundsDownToZero(t *testing.T) {
	t0 := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	// A single Lead↔R roundtrip (2 hops), then an interjection: 2/2 = 1, not
	// 0 — floor division on an even number keeps a sliver, exactly the
	// "higher bar, not no bar" the PRD describes.
	h := []Hop{
		{FromAgent: agLead, ToAgent: agR, At: t0},
		{FromAgent: agR, ToAgent: agLead, At: t0.Add(time.Minute)},
		{FromAgent: agW, ToAgent: agLead, At: t0.Add(2 * time.Minute)},
	}
	next := Hop{FromAgent: agLead, ToAgent: agR, At: t0.Add(3 * time.Minute)}
	if got := pairRoundtrips(h, next); got != 1 {
		t.Errorf("pairRoundtrips = %d, want 1 (1 hop survives the halving of 2, plus next)", got)
	}
}

func TestPairRoundtripsWindowIgnoresInterjectionsAndHumans(t *testing.T) {
	t0 := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	// 6 Lead↔R roundtrips (12 hops) with a THIRD AGENT interjecting after
	// every one AND a human message in the middle — the window counter must
	// not care about either: only the clock matters.
	var h []Hop
	at := t0
	step := func(d time.Duration) time.Time { at = at.Add(d); return at }
	for i := 0; i < 6; i++ {
		h = append(h,
			Hop{FromAgent: agLead, ToAgent: agR, At: step(time.Minute)},
			Hop{FromAgent: agR, ToAgent: agLead, At: step(time.Minute)},
			Hop{FromAgent: agW, ToAgent: agLead, At: step(time.Minute)},
		)
		if i == 2 {
			h = append(h, Hop{FromAgent: uuid.Nil, ToAgent: agLead, At: step(time.Minute)}) // human
		}
	}
	next := Hop{FromAgent: agLead, ToAgent: agR, At: step(time.Minute)}
	now := at
	// 6 roundtrips + next = 13 matching hops → (13+1)/2 = 7, regardless of
	// the interjections and the human in between.
	if got := pairRoundtripsWindow(h, next, now, 30*time.Minute); got != 7 {
		t.Errorf("pairRoundtripsWindow = %d, want 7 — interjections and a human must not reduce it (that independence is the whole point)", got)
	}
}

func TestPairRoundtripsWindowDropsOnlyWhatAgedOut(t *testing.T) {
	t0 := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	h := []Hop{
		{FromAgent: agLead, ToAgent: agR, At: t0},                       // 40m before now: outside a 30m window
		{FromAgent: agR, ToAgent: agLead, At: t0.Add(time.Minute)},      // 39m before: outside
		{FromAgent: agLead, ToAgent: agR, At: t0.Add(35 * time.Minute)}, // 5m before: inside
	}
	now := t0.Add(40 * time.Minute)
	next := Hop{FromAgent: agR, ToAgent: agLead, At: now}
	// Only the 35m hop plus next are inside the 30-minute window → 2 hops →
	// (2+1)/2 = 1.
	if got := pairRoundtripsWindow(h, next, now, 30*time.Minute); got != 1 {
		t.Errorf("pairRoundtripsWindow = %d, want 1 — only the hop still inside the window plus next should count", got)
	}
}

func TestPairRoundtripsWindowDisjointPairsDoNotCount(t *testing.T) {
	t0 := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	// A busy room where Lead↔W also roundtrips a lot — none of that is the
	// Lead↔R pair the window counter is asked about.
	h := []Hop{
		{FromAgent: agLead, ToAgent: agW, At: t0},
		{FromAgent: agW, ToAgent: agLead, At: t0.Add(time.Minute)},
		{FromAgent: agLead, ToAgent: agW, At: t0.Add(2 * time.Minute)},
	}
	next := Hop{FromAgent: agLead, ToAgent: agR, At: t0.Add(3 * time.Minute)}
	if got := pairRoundtripsWindow(h, next, next.At, 30*time.Minute); got != 1 {
		t.Errorf("pairRoundtripsWindow = %d, want 1 (next alone) — a different pair's traffic must not leak in", got)
	}
}

func TestPairRoundtripsWindowHumanNextIsZero(t *testing.T) {
	// Mirrors pairRoundtrips' own rule: a human trigger is never itself
	// limited (it is what resets/doesn't-reset the other counters).
	next := Hop{ToAgent: agLead, At: time.Now()}
	if got := pairRoundtripsWindow(nil, next, next.At, 30*time.Minute); got != 0 {
		t.Errorf("pairRoundtripsWindow(human next) = %d, want 0", got)
	}
}

func TestCheckLoopLimitsPairRoundtripsWindowTripsIndependentlyOfDecay(t *testing.T) {
	t0 := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	lim := Limits{
		MaxChainDepth: 100, MaxHopsPerHour: 10000, MaxPairRoundtrips: 100, // decaying limit effectively off
		MaxPairRoundtripsWindow: 5, MaxPairRoundtripsWindowDuration: 30 * time.Minute,
	}
	// 5 Lead↔R roundtrips (10 hops), each followed by a W interjection — the
	// decaying counter never gets anywhere near 100, but the window counter
	// (immune to the interjections) climbs on the raw clock and must still
	// trip the room.
	var h []Hop
	at := t0
	step := func(d time.Duration) time.Time { at = at.Add(d); return at }
	for i := 0; i < 5; i++ {
		h = append(h,
			Hop{FromAgent: agLead, ToAgent: agR, At: step(time.Minute)},
			Hop{FromAgent: agR, ToAgent: agLead, At: step(time.Minute)},
			Hop{FromAgent: agW, ToAgent: agLead, At: step(time.Minute)},
		)
	}
	next := Hop{FromAgent: agLead, ToAgent: agR, At: step(time.Minute)}
	v := CheckLoopLimits(h, next, lim, next.At)
	if v.Allowed {
		t.Fatalf("verdict = allowed, want paused — the window counter (immune to interjections) should have tripped: %+v", v)
	}
	if v.Detail != DetailPairRoundtripsWindow {
		t.Errorf("detail = %q, want %q", v.Detail, DetailPairRoundtripsWindow)
	}
	if v.PairRoundtripsWindow != v.LimitCount() {
		t.Errorf("LimitCount() = %d, want PairRoundtripsWindow %d", v.LimitCount(), v.PairRoundtripsWindow)
	}
}

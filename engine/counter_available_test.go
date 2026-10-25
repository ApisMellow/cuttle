package engine

import (
	"testing"

	"github.com/ApisMellow/cuttle/card"
)

// CounterAvailable is the public "the opponent's last turn can be countered
// right now" flag (cuttle-web issue #93). It must not depend on any hand:
// twin states that differ only by whether a player holds an unfrozen 2 (same
// card count) give the same value.

// wantTwinsAvailable asserts CounterAvailable equals want in both twins.
func wantTwinsAvailable(t *testing.T, withTwo, without GameState, want bool) {
	t.Helper()
	a, b := CounterAvailable(withTwo), CounterAvailable(without)
	if a != b {
		t.Fatalf("twins differ: with a 2 = %v, without = %v", a, b)
	}
	if a != want {
		t.Errorf("CounterAvailable = %v, want %v (phase=%v active=%v)", a, want, withTwo.Phase, withTwo.Active)
	}
}

func TestCounterAvailable_HandIndependentTwins(t *testing.T) {
	twin := pc(card.Seven, card.Clubs)
	ace := pc(card.Ace, card.Spades)
	four := pc(card.Four, card.Spades)

	// P1 plays oneOff on its turn; P2 holds p2Card and is to move.
	afterOneOff := func(oneOff, p2Card card.Card) GameState {
		s := mustApply(t, rewindBase(oneOff, p2Card), Move{Kind: MoveDraw})
		return mustApply(t, s, Move{Kind: MoveOneOff, Card: oneOff, HandIndex: 0})
	}

	t.Run("Phase0AfterOneOff", func(t *testing.T) {
		a, b := afterOneOff(ace, crTwoC), afterOneOff(ace, twin)
		if a.Phase != PhaseNormal || b.Phase != PhaseNormal {
			t.Fatalf("setup: phases %v / %v, want PhaseNormal", a.Phase, b.Phase)
		}
		wantTwinsAvailable(t, a, b, true)
	})

	t.Run("Phase3AfterFour", func(t *testing.T) {
		a, b := afterOneOff(four, crTwoC), afterOneOff(four, twin)
		if a.Phase != PhaseAwaitingDiscard || a.Pending != nil || b.Phase != PhaseAwaitingDiscard || b.Pending != nil {
			t.Fatalf("setup: want the new-style owed discard, got %v / %v", a.Phase, b.Phase)
		}
		wantTwinsAvailable(t, a, b, true)
	})

	t.Run("Phase5PendingWin", func(t *testing.T) {
		// Phase 5 is only entered when P2 holds a 2 (the accepted leak, design
		// §4), so the twin swaps that 2 for a 7 in place in the same state.
		a := popWin(t, pwBase(t, []card.Card{pwTwoS, crTenD}, []card.Card{crTwoC, crTenS}))
		if a.Phase != PhasePendingWin {
			t.Fatalf("setup: phase %v, want PhasePendingWin", a.Phase)
		}
		b := clone(a)
		b.Players[P2].Hand[handIndexOf(b.Players[P2].Hand, crTwoC)] = twin
		wantTwinsAvailable(t, a, b, true)
	})

	t.Run("AfterFirstMove", func(t *testing.T) {
		// Paying the owed discard is P2's first move: the 4's turn stands.
		pay := func(s GameState) GameState {
			h := s.Players[P2].Hand
			return mustApply(t, s, Move{Kind: MoveDiscardPair, DiscardA: handIndexOf(h, crTenS), DiscardB: handIndexOf(h, crQH)})
		}
		a, b := pay(afterOneOff(four, crTwoC)), pay(afterOneOff(four, twin))
		if a.Active != P2 || a.Phase != PhaseNormal {
			t.Fatalf("setup: P2 should play on in PhaseNormal, phase=%v active=%v", a.Phase, a.Active)
		}
		wantTwinsAvailable(t, a, b, false)
	})

	t.Run("FirstTurn", func(t *testing.T) {
		a := twoPlayerStart([]card.Card{crTwoC, ace}, []card.Card{crTenS}, []card.Card{crQH})
		b := twoPlayerStart([]card.Card{twin, ace}, []card.Card{crTenS}, []card.Card{crQH})
		wantTwinsAvailable(t, a, b, false)
	})

	t.Run("AfterCounter", func(t *testing.T) {
		// P2 counters P1's Ace, then plays 2♥ as a one-off on P1's glasses.
		// The twins differ by whether P1 holds a 2 (2♦ vs 9♥).
		twoH := pc(card.Two, card.Hearts)
		run := func(p1Extra card.Card) (mid, next GameState) {
			s := rewindBase(ace, crTwoC)
			s.Players[P1].Hand = []card.Card{ace, p1Extra}
			s.Players[P2].Hand = []card.Card{crTwoC, twoH}
			s = mustApply(t, s, Move{Kind: MoveDraw})
			s = mustApply(t, s, Move{Kind: MoveOneOff, Card: ace, HandIndex: 0})
			mid = mustCounter(t, s, crTwoC)
			tgt := Target{Owner: P1, Zone: ZonePermanents, Index: 0}
			next = mustApply(t, mid, Move{Kind: MoveOneOff, Card: twoH,
				HandIndex: handIndexOf(mid.Players[P2].Hand, twoH), Target: &tgt})
			return mid, next
		}
		aMid, aNext := run(pc(card.Two, card.Diamonds))
		bMid, bNext := run(pc(card.Nine, card.Hearts))
		// Within the countering turn: the counter was the first move.
		wantTwinsAvailable(t, aMid, bMid, false)
		// P1's next turn start: P2's counter turn is counterable.
		if aNext.Active != P1 || aNext.Phase != PhaseNormal {
			t.Fatalf("setup: want P1 to move in PhaseNormal, phase=%v active=%v", aNext.Phase, aNext.Active)
		}
		wantTwinsAvailable(t, aNext, bNext, true)
	})
}

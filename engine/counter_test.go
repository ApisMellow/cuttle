package engine

import (
	"testing"

	"github.com/ApisMellow/cuttle/card"
)

// Task 14: counter logic via PhaseAwaitingCounter. Since v0.3.0 new play
// never enters that phase (one-offs take effect at once; see
// counter_rewind_test.go); these tests cover the legacy path from a phase-1
// state built directly, as a v0.2.x save made mid-window would hold.

// openLegacyWindow builds the v0.2.x phase-1 state that one-off m used to
// open when the opponent held an unfrozen 2.
func openLegacyWindow(t *testing.T, s GameState, m Move) GameState {
	t.Helper()
	out := clone(s)
	p := &out.Players[out.Active]
	if m.HandIndex < 0 || m.HandIndex >= len(p.Hand) || p.Hand[m.HandIndex] != m.Card {
		t.Fatalf("openLegacyWindow: %v not at hand[%d]", m.Card, m.HandIndex)
	}
	removeFromHand(p, m.HandIndex)
	out.PassesInARow = 0
	out.Pending = &PendingOneOff{
		PlayedBy:   out.Active,
		Card:       m.Card,
		Target:     m.Target,
		ScrapIndex: m.ScrapIndex,
	}
	out.Phase = PhaseAwaitingCounter
	out.Active = out.Active.Other()
	return out
}

func hasCounterMove(moves []Move, idx int) bool {
	for _, m := range moves {
		if m.Kind == MoveCounter && m.HandIndex == idx {
			return true
		}
	}
	return false
}

func hasDecline(moves []Move) bool {
	for _, m := range moves {
		if m.Kind == MoveDecline {
			return true
		}
	}
	return false
}

func TestCounter_AceVsNoTwos_ResolvesImmediately(t *testing.T) {
	ace := card.Card{Rank: card.Ace, Suit: card.Spades}
	s := twoPlayerStart([]card.Card{ace}, nil, nil)
	s.Players[P2].Hand = []card.Card{{Rank: card.Five, Suit: card.Clubs}}
	s.Players[P1].Points = []PointEntry{
		{Card: card.Card{Rank: card.Seven, Suit: card.Hearts}, Owner: P1},
	}
	s2, err := Apply(s, Move{Kind: MoveOneOff, Card: ace, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	if s2.Phase != PhaseNormal {
		t.Errorf("expected PhaseNormal, got %d", s2.Phase)
	}
	if len(s2.Players[P1].Points) != 0 {
		t.Error("ace should have cleared points immediately")
	}
	if s2.Active != P2 {
		t.Error("turn should advance to P2")
	}
}

func TestCounter_AceVsOneTwo_Cancelled(t *testing.T) {
	ace := card.Card{Rank: card.Ace, Suit: card.Spades}
	two := card.Card{Rank: card.Two, Suit: card.Hearts}
	s := twoPlayerStart([]card.Card{ace}, []card.Card{two}, nil)
	s.Players[P1].Points = []PointEntry{
		{Card: card.Card{Rank: card.Seven, Suit: card.Hearts}, Owner: P1},
	}

	s1 := openLegacyWindow(t, s, Move{Kind: MoveOneOff, Card: ace, HandIndex: 0})
	if s1.Phase != PhaseAwaitingCounter {
		t.Fatalf("expected PhaseAwaitingCounter, got %d", s1.Phase)
	}
	if s1.Active != P2 {
		t.Error("waiting player should be P2")
	}
	moves := LegalMoves(s1)
	if !hasDecline(moves) {
		t.Error("expected MoveDecline")
	}
	if !hasCounterMove(moves, 0) {
		t.Error("expected MoveCounter for P2's 2")
	}

	// P2 counters.
	s2, err := Apply(s1, Move{Kind: MoveCounter, Card: two, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	// P1 has no 2s → auto-resolve. Chain len = 1 (odd) → cancelled.
	if s2.Phase != PhaseNormal {
		t.Errorf("expected PhaseNormal after resolution, got %d", s2.Phase)
	}
	if len(s2.Players[P1].Points) != 1 {
		t.Error("ace should have been cancelled; points should remain")
	}
	// Ace + one 2 both scrapped.
	if len(s2.Scrap) != 2 {
		t.Errorf("expected 2 cards in scrap, got %d", len(s2.Scrap))
	}
	if s2.Active != P2 {
		t.Error("turn should advance to P2 after P1's (cancelled) action")
	}
	if len(s2.Players[P1].Hand) != 0 || len(s2.Players[P2].Hand) != 0 {
		t.Error("both cards should have left hands")
	}
}

func TestCounter_AceVsTwoThenTwo_Resolves(t *testing.T) {
	ace := card.Card{Rank: card.Ace, Suit: card.Spades}
	two1 := card.Card{Rank: card.Two, Suit: card.Hearts}
	two2 := card.Card{Rank: card.Two, Suit: card.Clubs}
	s := twoPlayerStart([]card.Card{ace, two2}, []card.Card{two1}, nil)
	s.Players[P1].Points = []PointEntry{
		{Card: card.Card{Rank: card.Seven, Suit: card.Hearts}, Owner: P1},
	}

	s1 := openLegacyWindow(t, s, Move{Kind: MoveOneOff, Card: ace, HandIndex: 0})
	// Now P1 hand is [two2]. P2 hand is [two1]. Waiting: P2.
	if s1.Active != P2 || s1.Phase != PhaseAwaitingCounter {
		t.Fatal("expected awaiting counter, P2 to act")
	}
	s2, err := Apply(s1, Move{Kind: MoveCounter, Card: two1, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	// P1 has two2, should now be waiting.
	if s2.Phase != PhaseAwaitingCounter {
		t.Fatalf("expected still awaiting counter, got phase %d", s2.Phase)
	}
	if s2.Active != P1 {
		t.Error("waiting player should flip to P1")
	}
	moves := LegalMoves(s2)
	if !hasCounterMove(moves, 0) {
		t.Error("P1 should have MoveCounter for its 2")
	}
	s3, err := Apply(s2, Move{Kind: MoveCounter, Card: two2, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	// P2 has no more 2s → auto-resolve. Chain len = 2 (even) → resolves.
	if s3.Phase != PhaseNormal {
		t.Fatalf("expected PhaseNormal, got %d", s3.Phase)
	}
	if len(s3.Players[P1].Points) != 0 {
		t.Error("ace should have resolved; points cleared")
	}
	// Scrap: ace + 2 twos + 7H = 4.
	if len(s3.Scrap) != 4 {
		t.Errorf("expected 4 scrap, got %d", len(s3.Scrap))
	}
	if s3.Active != P2 {
		t.Error("turn should advance to P2 after P1's action resolves")
	}
}

func TestCounter_Decline_ResolvesImmediately(t *testing.T) {
	ace := card.Card{Rank: card.Ace, Suit: card.Spades}
	two := card.Card{Rank: card.Two, Suit: card.Hearts}
	s := twoPlayerStart([]card.Card{ace}, []card.Card{two}, nil)
	s.Players[P1].Points = []PointEntry{
		{Card: card.Card{Rank: card.Seven, Suit: card.Hearts}, Owner: P1},
	}
	s1 := openLegacyWindow(t, s, Move{Kind: MoveOneOff, Card: ace, HandIndex: 0})
	s2, err := Apply(s1, Move{Kind: MoveDecline})
	if err != nil {
		t.Fatal(err)
	}
	// Chain len 0 → even → resolves.
	if s2.Phase != PhaseNormal {
		t.Fatalf("expected PhaseNormal, got %d", s2.Phase)
	}
	if len(s2.Players[P1].Points) != 0 {
		t.Error("ace should have resolved")
	}
	if len(s2.Scrap) != 2 {
		t.Errorf("expected ace + 7H in scrap, got %d", len(s2.Scrap))
	}
	// P2's 2 remains in hand.
	if len(s2.Players[P2].Hand) != 1 {
		t.Error("P2's 2 should still be in hand")
	}
	if s2.Active != P2 {
		t.Error("turn should advance to P2")
	}
}

func TestCounter_FrozenTwoCannotCounter(t *testing.T) {
	ace := card.Card{Rank: card.Ace, Suit: card.Spades}
	two := card.Card{Rank: card.Two, Suit: card.Hearts}
	s := twoPlayerStart([]card.Card{ace}, []card.Card{two}, nil)
	s.Players[P2].FrozenIDs = map[int]bool{0: true}
	s.Players[P1].Points = []PointEntry{
		{Card: card.Card{Rank: card.Seven, Suit: card.Hearts}, Owner: P1},
	}
	// Frozen 2: transition to awaiting counter should be skipped entirely
	// (no legal counters), resolving immediately.
	s2, err := Apply(s, Move{Kind: MoveOneOff, Card: ace, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	if s2.Phase != PhaseNormal {
		t.Errorf("expected immediate resolution when opponent's 2 is frozen, got phase %d", s2.Phase)
	}
	if len(s2.Players[P1].Points) != 0 {
		t.Error("ace should have resolved immediately")
	}
}

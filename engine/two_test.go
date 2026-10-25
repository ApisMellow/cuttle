package engine

import (
	"testing"

	"github.com/ApisMellow/cuttle/card"
)

// Task 20: Two-as-scrap (royal target mode).
//
// A 2 played from PhaseNormal with a target scraps a single royal
// (Q/K), glasses-8, or Jack from a point stack, on either side of
// the field. Queen protection: if the target's owner has a Queen in
// permanents, only an opposing Queen may be targeted.

func hasTwoScrap(moves []Move, owner PlayerID, zone TargetZone, idx int) bool {
	for _, m := range moves {
		if m.Kind != MoveOneOff || m.Card.Rank != card.Two {
			continue
		}
		if m.Target == nil {
			continue
		}
		if m.Target.Owner == owner && m.Target.Zone == zone && m.Target.Index == idx {
			return true
		}
	}
	return false
}

func TestTwoScrap_LegalMovesEnumeratesRoyalsAndGlasses(t *testing.T) {
	two := card.Card{Rank: card.Two, Suit: card.Spades}
	s := twoPlayerStart([]card.Card{two}, nil, nil)
	// P1 has its own King too; P2 has a Queen and an 8 glasses.
	s.Players[P1].Permanents = []card.Card{{Rank: card.King, Suit: card.Clubs}}
	s.Players[P2].Permanents = []card.Card{
		{Rank: card.Queen, Suit: card.Diamonds},
		{Rank: card.Eight, Suit: card.Hearts},
	}
	moves := LegalMoves(s)
	// Own King: allowed (weird but spec: either side).
	if !hasTwoScrap(moves, P1, ZonePermanents, 0) {
		t.Error("expected 2-scrap on P1's own King")
	}
	// Opp Queen: allowed (queen doesn't protect itself).
	if !hasTwoScrap(moves, P2, ZonePermanents, 0) {
		t.Error("expected 2-scrap on opp Queen")
	}
	// Opp glasses-8: queen on opp side protects it.
	if hasTwoScrap(moves, P2, ZonePermanents, 1) {
		t.Error("opp glasses-8 should be protected by opp Queen")
	}
}

func TestTwoScrap_ScrapOpponentKing(t *testing.T) {
	two := card.Card{Rank: card.Two, Suit: card.Spades}
	king := card.Card{Rank: card.King, Suit: card.Clubs}
	s := twoPlayerStart([]card.Card{two}, nil, nil)
	s.Players[P2].Permanents = []card.Card{king}

	tgt := Target{Owner: P2, Zone: ZonePermanents, Index: 0}
	s2, err := Apply(s, Move{Kind: MoveOneOff, Card: two, HandIndex: 0, Target: &tgt})
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.Players[P2].Permanents) != 0 {
		t.Error("King should be scrapped")
	}
	// 2 + king in scrap.
	if len(s2.Scrap) != 2 {
		t.Errorf("expected 2 cards in scrap, got %d", len(s2.Scrap))
	}
	if s2.Active != P2 {
		t.Error("turn should advance to P2")
	}
}

func TestTwoScrap_ScrapOpponentQueenAllowed(t *testing.T) {
	two := card.Card{Rank: card.Two, Suit: card.Spades}
	queen := card.Card{Rank: card.Queen, Suit: card.Clubs}
	s := twoPlayerStart([]card.Card{two}, nil, nil)
	s.Players[P2].Permanents = []card.Card{queen}

	tgt := Target{Owner: P2, Zone: ZonePermanents, Index: 0}
	s2, err := Apply(s, Move{Kind: MoveOneOff, Card: two, HandIndex: 0, Target: &tgt})
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.Players[P2].Permanents) != 0 {
		t.Error("Queen should be scrapped (queen does not protect itself)")
	}
}

func TestTwoScrap_JackReclaimsPoint(t *testing.T) {
	two := card.Card{Rank: card.Two, Suit: card.Spades}
	jack := card.Card{Rank: card.Jack, Suit: card.Clubs}
	sevenH := card.Card{Rank: card.Seven, Suit: card.Hearts}
	s := twoPlayerStart([]card.Card{two}, nil, nil)
	// P1 controls a 7 that originally belonged to P2, via a Jack owned by P1.
	s.Players[P1].Points = []PointEntry{
		{Card: sevenH, Owner: P2, JackStack: []card.Card{jack}, JackOwners: []PlayerID{P1}},
	}
	// P2 plays a 2 targeting that Jack-controlled point to reclaim.
	s.Players[P1].Hand = nil
	s.Players[P2].Hand = []card.Card{two}
	s.Active = P2

	tgt := Target{Owner: P1, Zone: ZonePoints, Index: 0}
	s2, err := Apply(s, Move{Kind: MoveOneOff, Card: two, HandIndex: 0, Target: &tgt})
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.Players[P1].Points) != 0 {
		t.Error("point should have left P1's side")
	}
	if len(s2.Players[P2].Points) != 1 {
		t.Fatalf("point should have returned to P2 (original owner), got %d", len(s2.Players[P2].Points))
	}
	pe := s2.Players[P2].Points[0]
	if pe.Card != sevenH {
		t.Error("wrong point returned")
	}
	if len(pe.JackStack) != 0 {
		t.Error("Jack stack should be empty")
	}
	// 2 + jack in scrap.
	if len(s2.Scrap) != 2 {
		t.Errorf("expected 2 cards in scrap, got %d", len(s2.Scrap))
	}
}

func TestTwoScrap_CounterChain(t *testing.T) {
	// P1 plays 2-as-scrap on P2's King; P2 counters with its own 2.
	// Chain len 1 (odd) → cancelled; King survives.
	twoA := card.Card{Rank: card.Two, Suit: card.Spades}
	twoB := card.Card{Rank: card.Two, Suit: card.Hearts}
	king := card.Card{Rank: card.King, Suit: card.Clubs}
	s := twoPlayerStart([]card.Card{twoA}, []card.Card{twoB}, nil)
	s.Players[P2].Permanents = []card.Card{king}

	tgt := Target{Owner: P2, Zone: ZonePermanents, Index: 0}
	// Legacy phase-1 path (v0.2.x mid-window save); see counter_test.go.
	s1 := openLegacyWindow(t, s, Move{Kind: MoveOneOff, Card: twoA, HandIndex: 0, Target: &tgt})
	if s1.Phase != PhaseAwaitingCounter {
		t.Fatalf("expected PhaseAwaitingCounter, got %d", s1.Phase)
	}
	if s1.Active != P2 {
		t.Error("waiting player should be P2")
	}
	s2, err := Apply(s1, Move{Kind: MoveCounter, Card: twoB, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	// P1 has no more 2s → auto-resolve. Chain len 1 → cancelled.
	if s2.Phase != PhaseNormal {
		t.Fatalf("expected PhaseNormal, got %d", s2.Phase)
	}
	if len(s2.Players[P2].Permanents) != 1 {
		t.Error("King should survive cancelled 2-scrap")
	}
	if len(s2.Scrap) != 2 {
		t.Errorf("expected 2 cards in scrap (both 2s), got %d", len(s2.Scrap))
	}
}

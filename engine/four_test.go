package engine

import (
	"testing"

	"github.com/ApisMellow/cuttle/card"
)

// Task 17: Four one-off — opponent discards 2 cards. Since v0.3.0 the 4 ends
// P1's turn with Owed[P2] = 2, and P2 pays at the start of their own turn
// (PhaseAwaitingDiscard, no Pending), then plays on. See owed_discard_test.go
// for the counter sequences.

func TestFour_FiveCardOpponentHand_EnumeratesAllUnorderedPairs(t *testing.T) {
	four := card.Card{Rank: card.Four, Suit: card.Spades}
	s := twoPlayerStart([]card.Card{four}, nil, nil)
	s.Players[P2].Hand = []card.Card{
		{Rank: card.Five, Suit: card.Clubs},
		{Rank: card.Seven, Suit: card.Hearts},
		{Rank: card.Eight, Suit: card.Diamonds},
		{Rank: card.Nine, Suit: card.Spades},
		{Rank: card.Ten, Suit: card.Clubs},
	}

	s2, err := Apply(s, Move{Kind: MoveOneOff, Card: four, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	if s2.Phase != PhaseAwaitingDiscard {
		t.Fatalf("expected PhaseAwaitingDiscard, got %v", s2.Phase)
	}
	if s2.Active != P2 {
		t.Fatalf("expected Active=P2 (discarding player), got %v", s2.Active)
	}
	if s2.Owed != [2]int{0, 2} || s2.Pending != nil || s2.TurnEnd[P1] == nil {
		t.Fatalf("P1's turn should end with P2 owing 2: owed=%v pending=%v", s2.Owed, s2.Pending)
	}

	moves := LegalMoves(s2)
	// C(5,2) = 10 unordered pairs
	if len(moves) != 10 {
		t.Fatalf("expected 10 discard-pair moves, got %d: %+v", len(moves), moves)
	}
	seen := map[[2]int]bool{}
	for _, m := range moves {
		if m.Kind != MoveDiscardPair {
			t.Errorf("expected MoveDiscardPair, got %v", m.Kind)
		}
		a, b := m.DiscardA, m.DiscardB
		if a >= b {
			t.Errorf("expected a<b (unordered pair), got %d,%d", a, b)
		}
		seen[[2]int{a, b}] = true
	}
	if len(seen) != 10 {
		t.Errorf("expected 10 distinct pairs, got %d", len(seen))
	}

	// Apply one pair: discard indices 0 and 2 (five and eight) → scrap.
	s3, err := Apply(s2, Move{Kind: MoveDiscardPair, DiscardA: 0, DiscardB: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(s3.Players[P2].Hand) != 3 {
		t.Errorf("expected P2 hand size 3 after discard, got %d", len(s3.Players[P2].Hand))
	}
	// Scrap should contain 4 (the one-off) plus the two discards.
	if len(s3.Scrap) != 3 {
		t.Errorf("expected scrap size 3, got %d: %+v", len(s3.Scrap), s3.Scrap)
	}
	if s3.Phase != PhaseNormal {
		t.Errorf("expected PhaseNormal after discard resolved, got %v", s3.Phase)
	}
	// Paying is the start of P2's own turn, which goes on.
	if s3.Active != P2 || s3.Owed != [2]int{} || s3.TurnEnd != s2.TurnEnd {
		t.Errorf("expected P2's turn to continue with the debt paid, active=%v owed=%v", s3.Active, s3.Owed)
	}
}

func TestFour_OneCardOpponentHand_SingleDiscard(t *testing.T) {
	four := card.Card{Rank: card.Four, Suit: card.Spades}
	s := twoPlayerStart([]card.Card{four}, nil, nil)
	lone := card.Card{Rank: card.Seven, Suit: card.Hearts}
	s.Players[P2].Hand = []card.Card{lone}

	s2, err := Apply(s, Move{Kind: MoveOneOff, Card: four, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	if s2.Phase != PhaseAwaitingDiscard {
		t.Fatalf("expected PhaseAwaitingDiscard, got %v", s2.Phase)
	}
	moves := LegalMoves(s2)
	if len(moves) != 1 {
		t.Fatalf("expected 1 discard move (single-card hand), got %d", len(moves))
	}
	m := moves[0]
	if m.Kind != MoveDiscardPair || m.DiscardA != 0 || m.DiscardB != -1 {
		t.Errorf("expected single-card discard with DiscardA=0, DiscardB=-1, got %+v", m)
	}

	s3, err := Apply(s2, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(s3.Players[P2].Hand) != 0 {
		t.Errorf("expected empty hand after discard, got %+v", s3.Players[P2].Hand)
	}
	// Scrap: four + one discard
	if len(s3.Scrap) != 2 {
		t.Errorf("expected scrap size 2, got %d", len(s3.Scrap))
	}
	if s3.Phase != PhaseNormal || s3.Active != P2 || s3.Owed != [2]int{} {
		t.Errorf("expected P2's normal turn with no debt, got phase=%v active=%v owed=%v", s3.Phase, s3.Active, s3.Owed)
	}
}

func TestFour_ZeroCardOpponentHand_AutoResume(t *testing.T) {
	four := card.Card{Rank: card.Four, Suit: card.Spades}
	s := twoPlayerStart([]card.Card{four}, nil, nil)
	s.Players[P2].Hand = nil

	s2, err := Apply(s, Move{Kind: MoveOneOff, Card: four, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	// Empty hand → pays nothing; the debt clears at P2's turn start.
	if s2.Phase != PhaseNormal || s2.Owed != [2]int{} {
		t.Errorf("expected PhaseNormal with no debt (empty hand), got %v owed=%v", s2.Phase, s2.Owed)
	}
	// The 4 should be in scrap.
	if len(s2.Scrap) != 1 || s2.Scrap[0] != four {
		t.Errorf("expected scrap=[four], got %+v", s2.Scrap)
	}
	// Turn ended; active flips to P2.
	if s2.Active != P2 {
		t.Errorf("expected Active=P2 after turn end, got %v", s2.Active)
	}
}

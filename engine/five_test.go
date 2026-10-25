package engine

import (
	"testing"

	"github.com/ApisMellow/cuttle/card"
)

// Task 18: Five one-off — draw up to 2 cards (respecting 8-card hand limit
// and deck size). Counter flow applies as with any other one-off.

func TestFive_StandardDrawTwo(t *testing.T) {
	five := card.Card{Rank: card.Five, Suit: card.Spades}
	top1 := card.Card{Rank: card.Seven, Suit: card.Hearts}
	top2 := card.Card{Rank: card.Eight, Suit: card.Clubs}
	rest := card.Card{Rank: card.Nine, Suit: card.Diamonds}
	s := twoPlayerStart([]card.Card{five}, nil, []card.Card{top1, top2, rest})

	s2, err := Apply(s, Move{Kind: MoveOneOff, Card: five, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	if s2.Phase != PhaseNormal {
		t.Fatalf("expected PhaseNormal after resolve, got %v", s2.Phase)
	}
	// P1 had 1 card (the 5), played it (0), drew 2 → hand size 2.
	h := s2.Players[P1].Hand
	if len(h) != 2 || h[0] != top1 || h[1] != top2 {
		t.Errorf("expected P1 hand [top1, top2], got %+v", h)
	}
	if len(s2.Deck) != 1 || s2.Deck[0] != rest {
		t.Errorf("expected deck [rest], got %+v", s2.Deck)
	}
	if len(s2.Scrap) != 1 || s2.Scrap[0] != five {
		t.Errorf("expected scrap=[five], got %+v", s2.Scrap)
	}
	if s2.Active != P2 {
		t.Errorf("expected turn to advance to P2, got %v", s2.Active)
	}
}

func TestFive_DeckEmpty_DrawsZero(t *testing.T) {
	five := card.Card{Rank: card.Five, Suit: card.Spades}
	s := twoPlayerStart([]card.Card{five}, nil, nil)

	s2, err := Apply(s, Move{Kind: MoveOneOff, Card: five, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	if s2.Phase != PhaseNormal {
		t.Fatalf("expected PhaseNormal, got %v", s2.Phase)
	}
	if len(s2.Players[P1].Hand) != 0 {
		t.Errorf("expected empty hand, got %+v", s2.Players[P1].Hand)
	}
	if len(s2.Deck) != 0 {
		t.Errorf("expected empty deck, got %+v", s2.Deck)
	}
	if len(s2.Scrap) != 1 || s2.Scrap[0] != five {
		t.Errorf("expected scrap=[five], got %+v", s2.Scrap)
	}
	if s2.Active != P2 {
		t.Errorf("expected Active=P2, got %v", s2.Active)
	}
}

func TestFive_DeckHasOneCard_DrawsOne(t *testing.T) {
	five := card.Card{Rank: card.Five, Suit: card.Spades}
	only := card.Card{Rank: card.Seven, Suit: card.Hearts}
	s := twoPlayerStart([]card.Card{five}, nil, []card.Card{only})

	s2, err := Apply(s, Move{Kind: MoveOneOff, Card: five, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	h := s2.Players[P1].Hand
	if len(h) != 1 || h[0] != only {
		t.Errorf("expected hand=[only], got %+v", h)
	}
	if len(s2.Deck) != 0 {
		t.Errorf("expected empty deck, got %+v", s2.Deck)
	}
}

func TestFive_HandLimitCapsDraw(t *testing.T) {
	// P1 hand: 8 cards including a 5. After playing it, hand=7. Can draw 1
	// (reaching 8), not 2.
	five := card.Card{Rank: card.Five, Suit: card.Spades}
	hand := []card.Card{
		five,
		{Rank: card.Ten, Suit: card.Clubs},
		{Rank: card.Ten, Suit: card.Diamonds},
		{Rank: card.Ten, Suit: card.Hearts},
		{Rank: card.Nine, Suit: card.Clubs},
		{Rank: card.Nine, Suit: card.Diamonds},
		{Rank: card.Nine, Suit: card.Hearts},
		{Rank: card.Eight, Suit: card.Diamonds},
	}
	top1 := card.Card{Rank: card.Seven, Suit: card.Hearts}
	top2 := card.Card{Rank: card.Six, Suit: card.Clubs}
	s := twoPlayerStart(hand, nil, []card.Card{top1, top2})

	s2, err := Apply(s, Move{Kind: MoveOneOff, Card: five, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.Players[P1].Hand) != 8 {
		t.Errorf("expected hand size 8 (capped), got %d", len(s2.Players[P1].Hand))
	}
	// Only top1 was drawn; top2 remains on deck.
	if len(s2.Deck) != 1 || s2.Deck[0] != top2 {
		t.Errorf("expected deck=[top2], got %+v", s2.Deck)
	}
	last := s2.Players[P1].Hand[len(s2.Players[P1].Hand)-1]
	if last != top1 {
		t.Errorf("expected last drawn card=top1, got %v", last)
	}
}

func TestFive_HandFullAfterPlay_DrawsZero(t *testing.T) {
	// Hand of 9? Not possible — max 8. Edge: hand of 8 includes the 5.
	// After playing it, hand=7, draws 1. Covered above. This case: hand=8
	// (including 5) and deck empty → draws 0.
	five := card.Card{Rank: card.Five, Suit: card.Spades}
	hand := []card.Card{
		five,
		{Rank: card.Ten, Suit: card.Clubs},
		{Rank: card.Ten, Suit: card.Diamonds},
		{Rank: card.Ten, Suit: card.Hearts},
		{Rank: card.Nine, Suit: card.Clubs},
		{Rank: card.Nine, Suit: card.Diamonds},
		{Rank: card.Nine, Suit: card.Hearts},
		{Rank: card.Eight, Suit: card.Diamonds},
	}
	s := twoPlayerStart(hand, nil, nil)

	s2, err := Apply(s, Move{Kind: MoveOneOff, Card: five, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.Players[P1].Hand) != 7 {
		t.Errorf("expected hand size 7, got %d", len(s2.Players[P1].Hand))
	}
}

func TestFive_CounterFlow_TriggersAwaitingCounter(t *testing.T) {
	five := card.Card{Rank: card.Five, Suit: card.Spades}
	two := card.Card{Rank: card.Two, Suit: card.Hearts}
	s := twoPlayerStart([]card.Card{five}, []card.Card{two},
		[]card.Card{{Rank: card.Seven, Suit: card.Clubs}})

	// Legacy phase-1 path (v0.2.x mid-window save); see counter_test.go.
	s2 := openLegacyWindow(t, s, Move{Kind: MoveOneOff, Card: five, HandIndex: 0})
	if s2.Phase != PhaseAwaitingCounter {
		t.Fatalf("expected PhaseAwaitingCounter (P2 has a 2), got %v", s2.Phase)
	}
	if s2.Active != P2 {
		t.Errorf("expected Active=P2, got %v", s2.Active)
	}
	// Decline → effect resolves, P1 draws 1.
	s3, err := Apply(s2, Move{Kind: MoveDecline})
	if err != nil {
		t.Fatal(err)
	}
	if s3.Phase != PhaseNormal {
		t.Fatalf("expected PhaseNormal after decline, got %v", s3.Phase)
	}
	if len(s3.Players[P1].Hand) != 1 {
		t.Errorf("expected P1 hand size 1 (drew 1 from deck of 1), got %d", len(s3.Players[P1].Hand))
	}
}

func TestLegalMoves_FiveIsPlayableAsOneOff(t *testing.T) {
	five := card.Card{Rank: card.Five, Suit: card.Spades}
	s := twoPlayerStart([]card.Card{five}, nil, nil)
	moves := LegalMoves(s)
	found := false
	for _, m := range moves {
		if m.Kind == MoveOneOff && m.Card == five {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Five to be playable as one-off, got moves: %+v", moves)
	}
}

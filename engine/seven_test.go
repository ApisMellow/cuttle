package engine

import (
	"testing"

	"github.com/ApisMellow/cuttle/card"
)

// Task 21: Seven one-off — reveal top 2 deck cards, play one, return one.

func TestSeven_RevealsTwoAndEntersChoosing(t *testing.T) {
	seven := card.Card{Rank: card.Seven, Suit: card.Hearts}
	top1 := card.Card{Rank: card.Five, Suit: card.Clubs}
	top2 := card.Card{Rank: card.King, Suit: card.Spades}
	s := twoPlayerStart([]card.Card{seven}, nil, []card.Card{top1, top2, {Rank: card.Three, Suit: card.Diamonds}})

	s2, err := Apply(s, Move{Kind: MoveOneOff, Card: seven, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	if s2.Phase != PhaseSevenChoosing {
		t.Fatalf("expected PhaseSevenChoosing, got %v", s2.Phase)
	}
	if s2.Active != P1 {
		t.Errorf("active should remain P1, got %v", s2.Active)
	}
	if s2.Pending == nil || len(s2.Pending.Revealed) != 2 {
		t.Fatalf("expected 2 revealed, got %+v", s2.Pending)
	}
	if s2.Pending.Revealed[0] != top1 || s2.Pending.Revealed[1] != top2 {
		t.Errorf("revealed order wrong: %+v", s2.Pending.Revealed)
	}
	if len(s2.Deck) != 1 {
		t.Errorf("expected deck len 1, got %d", len(s2.Deck))
	}
	// Seven went to scrap.
	if len(s2.Scrap) != 1 || s2.Scrap[0] != seven {
		t.Errorf("expected seven in scrap, got %+v", s2.Scrap)
	}
}

func TestSeven_OneCardDeck_RevealsOne(t *testing.T) {
	seven := card.Card{Rank: card.Seven, Suit: card.Hearts}
	top1 := card.Card{Rank: card.Five, Suit: card.Clubs}
	s := twoPlayerStart([]card.Card{seven}, nil, []card.Card{top1})
	s2, err := Apply(s, Move{Kind: MoveOneOff, Card: seven, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	if s2.Phase != PhaseSevenChoosing {
		t.Fatalf("expected PhaseSevenChoosing, got %v", s2.Phase)
	}
	if len(s2.Pending.Revealed) != 1 {
		t.Fatalf("expected 1 revealed, got %+v", s2.Pending.Revealed)
	}
	if len(s2.Deck) != 0 {
		t.Errorf("expected empty deck, got %d", len(s2.Deck))
	}
}

func TestSeven_EmptyDeck_NotLegal(t *testing.T) {
	seven := card.Card{Rank: card.Seven, Suit: card.Hearts}
	s := twoPlayerStart([]card.Card{seven}, nil, nil)
	moves := LegalMoves(s)
	for _, m := range moves {
		if m.Kind == MoveOneOff && m.Card.Rank == card.Seven {
			t.Errorf("seven one-off should not be legal with empty deck")
		}
	}
}

func TestSeven_LegalMovesInChoosingWrapInnerMoves(t *testing.T) {
	seven := card.Card{Rank: card.Seven, Suit: card.Hearts}
	five := card.Card{Rank: card.Five, Suit: card.Clubs}
	king := card.Card{Rank: card.King, Suit: card.Spades}
	s := twoPlayerStart([]card.Card{seven}, nil, []card.Card{five, king, {Rank: card.Three, Suit: card.Diamonds}})
	s2, _ := Apply(s, Move{Kind: MoveOneOff, Card: seven, HandIndex: 0})

	moves := LegalMoves(s2)
	if len(moves) == 0 {
		t.Fatal("expected some SevenPick moves")
	}
	var sawFivePoint, sawFiveOneOff, sawKingPerm bool
	for _, m := range moves {
		if m.Kind != MoveSevenPick {
			t.Errorf("expected only MoveSevenPick, got %v", m.Kind)
			continue
		}
		if m.SubMove == nil {
			t.Errorf("missing submove")
			continue
		}
		if m.SubMove.Kind == MoveDraw || m.SubMove.Kind == MovePass {
			t.Errorf("sub-move should not be draw/pass, got %v", m.SubMove.Kind)
		}
		if m.Card == five && m.SubMove.Kind == MovePlayPoint {
			sawFivePoint = true
		}
		if m.Card == five && m.SubMove.Kind == MoveOneOff {
			sawFiveOneOff = true
		}
		if m.Card == king && m.SubMove.Kind == MovePlayPermanent {
			sawKingPerm = true
		}
	}
	if !sawFivePoint || !sawFiveOneOff || !sawKingPerm {
		t.Errorf("missing expected inner moves: fivePoint=%v fiveOneOff=%v kingPerm=%v", sawFivePoint, sawFiveOneOff, sawKingPerm)
	}
}

func TestSeven_PickPlayPointReturnsOther(t *testing.T) {
	seven := card.Card{Rank: card.Seven, Suit: card.Hearts}
	five := card.Card{Rank: card.Five, Suit: card.Clubs}
	king := card.Card{Rank: card.King, Suit: card.Spades}
	deckRest := card.Card{Rank: card.Three, Suit: card.Diamonds}
	s := twoPlayerStart([]card.Card{seven}, nil, []card.Card{five, king, deckRest})
	s2, _ := Apply(s, Move{Kind: MoveOneOff, Card: seven, HandIndex: 0})

	pick := Move{Kind: MoveSevenPick, Card: five, SubMove: &Move{Kind: MovePlayPoint, Card: five}}
	s3, err := Apply(s2, pick)
	if err != nil {
		t.Fatal(err)
	}
	if s3.Phase != PhaseNormal {
		t.Errorf("expected PhaseNormal, got %v", s3.Phase)
	}
	// Five played as a point card on P1.
	if len(s3.Players[P1].Points) != 1 || s3.Players[P1].Points[0].Card != five {
		t.Errorf("expected five on P1 points, got %+v", s3.Players[P1].Points)
	}
	// King returned to top of deck.
	if len(s3.Deck) != 2 || s3.Deck[0] != king || s3.Deck[1] != deckRest {
		t.Errorf("expected king on top of deck, got %+v", s3.Deck)
	}
	// Turn ended; P2 now active.
	if s3.Active != P2 {
		t.Errorf("expected P2 active after point play, got %v", s3.Active)
	}
	// Pending cleared.
	if s3.Pending != nil {
		t.Errorf("expected nil pending, got %+v", s3.Pending)
	}
}

func TestSeven_PickOneOffCanBeCountered(t *testing.T) {
	seven := card.Card{Rank: card.Seven, Suit: card.Hearts}
	ace := card.Card{Rank: card.Ace, Suit: card.Clubs}
	king := card.Card{Rank: card.King, Suit: card.Spades}
	two := card.Card{Rank: card.Two, Suit: card.Hearts}
	s := twoPlayerStart([]card.Card{seven}, []card.Card{two}, []card.Card{ace, king})
	// Note: opponent has a 2 but the 7 itself has a deck draw — opponent
	// will also be offered to counter the 7. For this test we want the 7
	// to resolve cleanly first, so give P1 no hand 2s; opponent still can
	// counter the 7 before reveal. Bypass by playing to a state that skips
	// the 7's counter window — easiest: start s directly in PhaseSevenChoosing.
	s2 := s
	s2.Phase = PhaseSevenChoosing
	s2.Pending = &PendingOneOff{PlayedBy: P1, Card: seven, Revealed: []card.Card{ace, king}}
	s2.Deck = nil
	s2.Scrap = []card.Card{seven}
	s2.Players[P1].Hand = nil

	pick := Move{Kind: MoveSevenPick, Card: ace, SubMove: &Move{Kind: MoveOneOff, Card: ace}}
	s3, err := Apply(s2, pick)
	if err != nil {
		t.Fatal(err)
	}
	// v0.3.0: the picked Ace resolves at once even though P2 holds a 2; the
	// turn is counterable, and P2 may counter it at the start of its turn.
	if s3.Phase != PhaseNormal || s3.Pending != nil {
		t.Errorf("expected PhaseNormal with no pending after picking ace, got phase=%v pending=%+v", s3.Phase, s3.Pending)
	}
	if s3.Active != P2 {
		t.Errorf("expected P2's turn, got %v", s3.Active)
	}
	// King was returned to top of deck before dispatch.
	if len(s3.Deck) != 1 || s3.Deck[0] != king {
		t.Errorf("expected king on deck, got %+v", s3.Deck)
	}
	// The pick came from the deck, so it is not a countered card; this state
	// was built in phase 2, so the 7 itself was never recorded either.
	if !s3.LastTurn.Counterable || len(s3.LastTurn.Countered) != 0 {
		t.Errorf("LastTurn = %+v, want counterable with no countered cards", s3.LastTurn)
	}
}

func TestSeven_UnchosenReturnsToDeckTop(t *testing.T) {
	seven := card.Card{Rank: card.Seven, Suit: card.Hearts}
	five := card.Card{Rank: card.Five, Suit: card.Clubs}
	king := card.Card{Rank: card.King, Suit: card.Spades}
	deckRest := card.Card{Rank: card.Three, Suit: card.Diamonds}
	s := twoPlayerStart([]card.Card{seven}, nil, []card.Card{five, king, deckRest})
	s2, _ := Apply(s, Move{Kind: MoveOneOff, Card: seven, HandIndex: 0})

	// Pick the king as a permanent; five should return to deck top.
	pick := Move{Kind: MoveSevenPick, Card: king, SubMove: &Move{Kind: MovePlayPermanent, Card: king}}
	s3, err := Apply(s2, pick)
	if err != nil {
		t.Fatal(err)
	}
	if len(s3.Deck) != 2 || s3.Deck[0] != five || s3.Deck[1] != deckRest {
		t.Errorf("expected five on top of deck, got %+v", s3.Deck)
	}
	if len(s3.Players[P1].Permanents) != 1 || s3.Players[P1].Permanents[0] != king {
		t.Errorf("expected king permanent, got %+v", s3.Players[P1].Permanents)
	}
}

// Regression: a frozen hand index must be remapped when the 7 leaves the
// hand, or the revealed card injected at the stale index is wrongly frozen
// (LegalMoves offered the pick but Apply rejected it).
func TestSeven_FrozenIndexRemappedWhenSevenPlayed(t *testing.T) {
	s := twoPlayerStart(
		[]card.Card{{Rank: card.Seven, Suit: card.Clubs}, {Rank: card.Five, Suit: card.Hearts}},
		nil,
		[]card.Card{{Rank: card.Eight, Suit: card.Spades}, {Rank: card.Three, Suit: card.Diamonds}},
	)
	s.Players[0].FrozenIDs = map[int]bool{1: true} // 5♥ frozen by an earlier 9
	next, err := Apply(s, Move{Kind: MoveOneOff, Card: card.Card{Rank: card.Seven, Suit: card.Clubs}, HandIndex: 0})
	if err != nil {
		t.Fatalf("playing 7: %v", err)
	}
	if next.Phase != PhaseSevenChoosing {
		t.Fatalf("phase = %d, want PhaseSevenChoosing", next.Phase)
	}
	if !next.Players[0].FrozenIDs[0] || next.Players[0].FrozenIDs[1] {
		t.Errorf("FrozenIDs = %v, want frozen mark remapped to index 0", next.Players[0].FrozenIDs)
	}
	// Every offered pick must be applicable; in particular playing the
	// revealed 8♠, which is injected at index 1 (the stale frozen index).
	moves := LegalMoves(next)
	if len(moves) == 0 {
		t.Fatal("no seven picks offered")
	}
	for _, m := range moves {
		if _, err := Apply(next, m); err != nil {
			t.Errorf("offered pick %q rejected: %v", m.Describe(next), err)
		}
	}
}

// Regression: when no revealed card has a legal play (here two Jacks with
// no opponent points to steal), the player scraps one revealed card and the
// other returns to the top of the deck; the game must not dead-end.
func TestSeven_NoLegalPlayForRevealed_ScrapsChosen(t *testing.T) {
	s := twoPlayerStart(
		[]card.Card{{Rank: card.Seven, Suit: card.Clubs}},
		nil,
		[]card.Card{{Rank: card.Jack, Suit: card.Spades}, {Rank: card.Jack, Suit: card.Hearts}},
	)
	next, err := Apply(s, Move{Kind: MoveOneOff, Card: card.Card{Rank: card.Seven, Suit: card.Clubs}, HandIndex: 0})
	if err != nil {
		t.Fatalf("playing 7: %v", err)
	}
	moves := LegalMoves(next)
	if len(moves) != 2 {
		t.Fatalf("got %d moves, want 2 scrap picks: %+v", len(moves), moves)
	}
	for _, m := range moves {
		if m.Kind != MoveSevenPick || m.SubMove != nil {
			t.Fatalf("expected scrap pick (nil SubMove), got %+v", m)
		}
	}
	end, err := Apply(next, moves[0]) // scrap J♠
	if err != nil {
		t.Fatalf("scrap pick: %v", err)
	}
	if end.Phase != PhaseNormal || end.Active != P2 {
		t.Errorf("phase/active = %d/P%d, want PhaseNormal/P2", end.Phase, end.Active+1)
	}
	if len(end.Scrap) != 2 || end.Scrap[1] != (card.Card{Rank: card.Jack, Suit: card.Spades}) {
		t.Errorf("scrap = %v, want [7♣ J♠]", end.Scrap)
	}
	if len(end.Deck) != 1 || end.Deck[0] != (card.Card{Rank: card.Jack, Suit: card.Hearts}) {
		t.Errorf("deck = %v, want [J♥] back on top", end.Deck)
	}
	// A scrap pick must be illegal when a revealed card IS playable.
	s2 := twoPlayerStart(
		[]card.Card{{Rank: card.Seven, Suit: card.Clubs}},
		nil,
		[]card.Card{{Rank: card.Eight, Suit: card.Spades}, {Rank: card.Jack, Suit: card.Hearts}},
	)
	next2, err := Apply(s2, Move{Kind: MoveOneOff, Card: card.Card{Rank: card.Seven, Suit: card.Clubs}, HandIndex: 0})
	if err != nil {
		t.Fatalf("playing 7: %v", err)
	}
	if _, err := Apply(next2, Move{Kind: MoveSevenPick, Card: card.Card{Rank: card.Jack, Suit: card.Hearts}}); err == nil {
		t.Error("scrap pick should be illegal while the revealed 8♠ is playable")
	}
}

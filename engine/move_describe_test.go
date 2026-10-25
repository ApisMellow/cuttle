package engine

import (
	"strings"
	"testing"

	"github.com/ApisMellow/cuttle/card"
)

func TestDescribe_PlayPoint(t *testing.T) {
	s := twoPlayerStart([]card.Card{{Rank: card.Nine, Suit: card.Hearts}}, nil, nil)
	m := Move{Kind: MovePlayPoint, Card: card.Card{Rank: card.Nine, Suit: card.Hearts}, HandIndex: 0}
	got := m.Describe(s)
	if !strings.Contains(got, "9♥") || !strings.Contains(strings.ToLower(got), "point") {
		t.Errorf("unexpected description: %q", got)
	}
}

func TestDescribe_Draw(t *testing.T) {
	s := twoPlayerStart(nil, nil, []card.Card{{Rank: card.Two, Suit: card.Clubs}})
	m := Move{Kind: MoveDraw}
	if !strings.Contains(strings.ToLower(m.Describe(s)), "draw") {
		t.Error("draw description should contain 'draw'")
	}
}

// TestDescribe_AllKinds exercises every MoveKind branch of Describe, so the
// REPL menu text can't silently regress.
func TestDescribe_AllKinds(t *testing.T) {
	s := twoPlayerStart(nil, nil, nil)
	s.Players[1].Points = []PointEntry{{Card: card.Card{Rank: card.Five, Suit: card.Clubs}, Owner: P2}}
	tgt := &Target{Owner: P2, Zone: ZonePoints, Index: 0}
	sub := &Move{Kind: MovePlayPoint, Card: card.Card{Rank: card.Ten, Suit: card.Hearts}}
	cases := []struct {
		name string
		m    Move
		want []string
	}{
		{"draw", Move{Kind: MoveDraw}, []string{"draw"}},
		{"pass", Move{Kind: MovePass}, []string{"pass"}},
		{"point", Move{Kind: MovePlayPoint, Card: card.Card{Rank: card.Nine, Suit: card.Hearts}}, []string{"9♥", "point"}},
		{"jack", Move{Kind: MovePlayPermanent, Card: card.Card{Rank: card.Jack, Suit: card.Spades}}, []string{"J♠", "steal"}},
		{"permanent", Move{Kind: MovePlayPermanent, Card: card.Card{Rank: card.Queen, Suit: card.Spades}}, []string{"Q♠", "permanent"}},
		{"scuttle", Move{Kind: MoveScuttle, Card: card.Card{Rank: card.Six, Suit: card.Hearts}, Target: tgt}, []string{"scuttle", "5♣", "6♥"}},
		{"scuttle no target", Move{Kind: MoveScuttle, Card: card.Card{Rank: card.Six, Suit: card.Hearts}}, []string{"scuttle", "6♥"}},
		{"one-off", Move{Kind: MoveOneOff, Card: card.Card{Rank: card.Ace, Suit: card.Clubs}}, []string{"A♣", "one-off"}},
		{"counter", Move{Kind: MoveCounter, Card: card.Card{Rank: card.Two, Suit: card.Diamonds}}, []string{"counter", "2♦"}},
		{"decline", Move{Kind: MoveDecline}, []string{"decline"}},
		{"seven sub", Move{Kind: MoveSevenPick, Card: sub.Card, SubMove: sub}, []string{"7:", "10♥", "point"}},
		{"seven scrap", Move{Kind: MoveSevenPick, Card: card.Card{Rank: card.Jack, Suit: card.Hearts}}, []string{"7:", "scrap", "J♥"}},
		{"discard", Move{Kind: MoveDiscardPair, DiscardA: 0, DiscardB: 2}, []string{"discard", "0", "2"}},
		{"unknown", Move{Kind: MoveKind(99)}, []string{"?"}},
	}
	for _, tc := range cases {
		got := tc.m.Describe(s)
		for _, w := range tc.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: Describe() = %q, want it to contain %q", tc.name, got, w)
			}
		}
	}
}

// Decline reads "Let it happen" at a pending win; the legacy phase-1 Decline
// keeps "decline to counter".
func TestDescribe_DeclineByPhase(t *testing.T) {
	s := twoPlayerStart(nil, nil, nil)
	s.Phase = PhasePendingWin
	if got := (Move{Kind: MoveDecline}).Describe(s); got != "Let it happen" {
		t.Errorf("pending win: %q", got)
	}
	s.Phase = PhaseAwaitingCounter
	if got := (Move{Kind: MoveDecline}).Describe(s); got != "decline to counter" {
		t.Errorf("legacy window: %q", got)
	}
}

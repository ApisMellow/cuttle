package render

import (
	"strings"
	"testing"

	"github.com/ApisMellow/cuttle/card"
	"github.com/ApisMellow/cuttle/engine"
)

func TestRender_BasicState(t *testing.T) {
	s := engine.GameState{
		Players: [2]engine.PlayerState{
			{
				Hand: []card.Card{
					{Rank: card.King, Suit: card.Spades},
					{Rank: card.Nine, Suit: card.Hearts},
				},
				Points:     []engine.PointEntry{{Card: card.Card{Rank: card.Seven, Suit: card.Hearts}}},
				Permanents: []card.Card{{Rank: card.Queen, Suit: card.Diamonds}},
			},
			{
				Hand:   []card.Card{{Rank: card.Ace, Suit: card.Clubs}},
				Points: []engine.PointEntry{{Card: card.Card{Rank: card.Ten, Suit: card.Diamonds}}},
			},
		},
		Deck:   make([]card.Card, 30),
		Active: engine.P1,
		Phase:  engine.PhaseNormal,
	}
	out := Render(s)
	for _, want := range []string{"PLAYER 1", "PLAYER 2", "K♠", "9♥", "Q♦", "7♥", "10♦", "P1 TO MOVE", "ONE-OFF EFFECTS", "scrap ALL point cards"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRender_TurnStartSteps(t *testing.T) {
	base := engine.GameState{Active: engine.P2, Deck: make([]card.Card, 3)}

	pw := base
	pw.Phase = engine.PhasePendingWin
	if out := Render(pw); !strings.Contains(out, "Not So Fast! P1 reached their threshold; P2 may counter or let it happen") {
		t.Errorf("pending win line missing:\n%s", out)
	}

	od := base
	od.Phase = engine.PhaseAwaitingDiscard
	od.Owed = [2]int{0, 2}
	out := Render(od)
	for _, want := range []string{"P2 starts the turn by paying the owed discard", "P2 owes 2 discards"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q:\n%s", want, out)
		}
	}

	// A debt waiting for a later turn start (after a counter-back) shows too.
	later := base
	later.Active = engine.P1
	later.Owed = [2]int{0, 2}
	if out := Render(later); !strings.Contains(out, "P2 owes 2 discards") || strings.Contains(out, "paying") {
		t.Errorf("waiting debt:\n%s", out)
	}
}

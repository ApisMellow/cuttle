package engine

import (
	"fmt"

	"github.com/ApisMellow/cuttle/card"
)

type MoveKind uint8

const (
	MoveDraw MoveKind = iota
	MovePlayPoint
	MovePlayPermanent
	MoveScuttle
	MoveOneOff
	MoveCounter
	MoveDecline
	MoveSevenPick
	MoveDiscardPair
	MovePass
)

type Move struct {
	Kind       MoveKind
	Card       card.Card // card from hand being played (or chosen, for 7)
	HandIndex  int       // index in active player's hand of Card
	Target     *Target
	JackTarget *Target // for MovePlayPermanent when Card is a Jack
	ScrapIndex int     // for MoveOneOff with rank 3 (which scrap card to take)
	DiscardA   int     // for MoveDiscardPair: hand indices to discard
	DiscardB   int
	SubMove    *Move // for MoveSevenPick
}

func (m Move) Describe(s GameState) string {
	switch m.Kind {
	case MoveDraw:
		return "draw a card"
	case MovePass:
		return "pass"
	case MovePlayPoint:
		return fmt.Sprintf("play %s as point card", m.Card)
	case MovePlayPermanent:
		if m.Card.Rank == card.Jack {
			return fmt.Sprintf("play %s (steal opponent point)", m.Card)
		}
		return fmt.Sprintf("play %s as permanent", m.Card)
	case MoveScuttle:
		if m.Target != nil {
			tc := s.Players[m.Target.Owner].Points[m.Target.Index].Card
			return fmt.Sprintf("scuttle opponent's %s with %s", tc, m.Card)
		}
		return fmt.Sprintf("scuttle with %s", m.Card)
	case MoveOneOff:
		return fmt.Sprintf("play %s as one-off", m.Card)
	case MoveCounter:
		return fmt.Sprintf("counter with %s", m.Card)
	case MoveDecline:
		if s.Phase == PhasePendingWin {
			return "Let it happen"
		}
		return "decline to counter"
	case MoveSevenPick:
		if m.SubMove != nil {
			return fmt.Sprintf("7: %s", m.SubMove.Describe(s))
		}
		return fmt.Sprintf("7: no legal play — scrap %s", m.Card)
	case MoveDiscardPair:
		return fmt.Sprintf("discard hand[%d] and hand[%d]", m.DiscardA, m.DiscardB)
	}
	return "?"
}

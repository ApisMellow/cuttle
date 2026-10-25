package engine

import (
	"reflect"
	"testing"

	"github.com/ApisMellow/cuttle/card"
)

// v0.3.0 pending win, "Not So Fast!" (cuttle-web docs/counter-rewind-design.md
// §4, §8 E6). A win on a counterable turn ends the mover's turn at once. If
// the opponent can counter (an unfrozen 2 and a position to restore), the
// game enters PhasePendingWin with the opponent active: they counter (the
// normal rewind) or decline ("Let it happen") and the mover wins. Otherwise
// the mover wins at once. A win on a turn that can't be countered is
// immediate, as before.

var (
	pwTwoS   = pc(card.Two, card.Spades)   // P1's pop 2
	pwTwoD   = pc(card.Two, card.Diamonds) // P1's counter 2
	pwTwoH   = pc(card.Two, card.Hearts)   // P2's second 2
	pwSevS   = pc(card.Seven, card.Spades)
	pwSevD   = pc(card.Seven, card.Diamonds) // deck, the 7's winning pick
	pwSevH   = pc(card.Seven, card.Hearts)   // P1's point under P2's Jack
	pwSevC   = pc(card.Seven, card.Clubs)    // twin card for a 2
	pwEightD = pc(card.Eight, card.Diamonds) // twin card for a 2
	pwNineS  = pc(card.Nine, card.Spades)
	pwFiveS  = pc(card.Five, card.Spades)
)

// pwBase: P2 is to move and opens with a draw (Q♥), so TurnEnd[P2] exists
// when P1 moves. P1 has 10♥ + 4♥ = 14 on its side; its 7♥ sits under P2's
// J♠ on P2's side (P2: 7♥ + 5♦ = 12). P1 reaching 21 takes the 7♥ back or
// any 7 more.
func pwBase(t *testing.T, p1Hand, p2Hand []card.Card) GameState {
	t.Helper()
	s := twoPlayerStart(p1Hand, p2Hand, []card.Card{crQH, pwSevD, crThrH, crKingH, crNineC})
	s.Players[P1].Points = []PointEntry{{Card: crTenH, Owner: P1}, {Card: pc(card.Four, card.Hearts), Owner: P1}}
	s.Players[P2].Points = []PointEntry{
		{Card: pwSevH, Owner: P1, JackStack: []card.Card{crJackS}, JackOwners: []PlayerID{P2}},
		{Card: crFivD, Owner: P2},
	}
	s.Scrap = []card.Card{crFourC}
	s.Active = P2
	return mustApply(t, s, Move{Kind: MoveDraw})
}

// popWin: P1's 2♠ pops P2's J♠ off the 7♥, which comes back to P1: 21.
func popWin(t *testing.T, s GameState) GameState {
	t.Helper()
	return mustApply(t, s, Move{Kind: MoveOneOff, Card: pwTwoS, HandIndex: handIndexOf(s.Players[P1].Hand, pwTwoS),
		Target: &Target{Owner: P2, Zone: ZonePoints, Index: 0}})
}

// sevenWin: P1's 7♠ reveals 7♦ and 3♥; the 7♦ played for points makes 21.
func sevenWin(t *testing.T, s GameState) GameState {
	t.Helper()
	s1 := mustApply(t, s, Move{Kind: MoveOneOff, Card: pwSevS, HandIndex: handIndexOf(s.Players[P1].Hand, pwSevS)})
	m, ok := findMove(LegalMoves(s1), func(m Move) bool {
		return m.Kind == MoveSevenPick && m.Card == pwSevD && m.SubMove.Kind == MovePlayPoint
	})
	if !ok {
		t.Fatalf("7♦ point pick not offered: %+v", LegalMoves(s1))
	}
	return mustApply(t, s1, m)
}

func wantPendingWin(t *testing.T, s GameState, decider PlayerID) {
	t.Helper()
	if s.Phase != PhasePendingWin || s.Active != decider || s.Winner != nil {
		t.Fatalf("want PhasePendingWin with P%d deciding, got phase=%v active=%v winner=%v", decider+1, s.Phase, s.Active, s.Winner)
	}
	if !HasWon(s.Players[decider.Other()]) {
		t.Errorf("the pending winner P%d should be at threshold", decider.Other()+1)
	}
}

func wantWinner(t *testing.T, s GameState, w PlayerID) {
	t.Helper()
	if s.Phase != PhaseGameOver || s.Winner == nil || *s.Winner != w || s.Active != w {
		t.Fatalf("want P%d to win (PhaseGameOver, winner active), got phase=%v winner=%v active=%v", w+1, s.Phase, s.Winner, s.Active)
	}
	if LegalMoves(s) != nil {
		t.Errorf("no moves after game over, got %+v", LegalMoves(s))
	}
}

// E6: the pending win is entered from every win source, but only when the
// opponent holds an unfrozen 2; with no 2 the game is over at once.
func TestPendingWin_E6_EverySource(t *testing.T) {
	type source struct {
		name string
		// run plays from a fresh base to the moment the mover's turn ends.
		// withTwo says whether the decider holds an unfrozen 2.
		run      func(t *testing.T, withTwo bool) GameState
		mover    PlayerID
		moverPts int
	}
	p2Hand := func(withTwo bool) []card.Card {
		if withTwo {
			return []card.Card{crTwoC, crTenS}
		}
		return []card.Card{pwSevC, crTenS}
	}
	sources := []source{
		{name: "OneOffItself", mover: P1, moverPts: 21, run: func(t *testing.T, withTwo bool) GameState {
			return popWin(t, pwBase(t, []card.Card{pwTwoS, crTenD}, p2Hand(withTwo)))
		}},
		{name: "SevenPick", mover: P1, moverPts: 21, run: func(t *testing.T, withTwo bool) GameState {
			return sevenWin(t, pwBase(t, []card.Card{pwSevS, crTenD}, p2Hand(withTwo)))
		}},
		{name: "CounterRestore", mover: P1, moverPts: 21, run: func(t *testing.T, withTwo bool) GameState {
			// P1 wins with the pop; P2 counters with 2♣ and draws; P1
			// counters back with 2♦, restoring its winning position. P2's
			// 2♣ is scrapped by that counter-back, so the decider's 2 is
			// the extra 2♥ (or the 7♣ twin).
			extra := pwSevC
			if withTwo {
				extra = pwTwoH
			}
			s := pwBase(t, []card.Card{pwTwoS, pwTwoD}, []card.Card{crTwoC, crTenS, extra})
			s = popWin(t, s)
			wantPendingWin(t, s, P2)
			s = mustCounter(t, s, crTwoC)
			s = mustApply(t, s, Move{Kind: MoveDraw})
			return mustCounter(t, s, pwTwoD)
		}},
		{name: "ActionAfterCounter", mover: P2, moverPts: 21, run: func(t *testing.T, withTwo bool) GameState {
			// P1 plays a non-winning 5; P2 counters and plays 9♠ for points:
			// 12 + 9 = 21 on a counterable turn. The decider is P1.
			p1Extra := pwEightD
			if withTwo {
				p1Extra = pwTwoD
			}
			s := pwBase(t, []card.Card{pwFiveS, p1Extra}, []card.Card{crTwoC, pwNineS})
			s = mustApply(t, s, Move{Kind: MoveOneOff, Card: pwFiveS, HandIndex: 0})
			s = mustCounter(t, s, crTwoC)
			return mustApply(t, s, Move{Kind: MovePlayPoint, Card: pwNineS, HandIndex: handIndexOf(s.Players[P2].Hand, pwNineS)})
		}},
	}
	for _, src := range sources {
		t.Run(src.name+"/WithTwo", func(t *testing.T) {
			s := src.run(t, true)
			if got := PointTotal(s.Players[src.mover]); got != src.moverPts {
				t.Fatalf("mover total = %d, want %d", got, src.moverPts)
			}
			wantPendingWin(t, s, src.mover.Other())
			if !s.LastTurn.Counterable {
				t.Errorf("the winning turn should be the counterable LastTurn")
			}
		})
		t.Run(src.name+"/NoTwo", func(t *testing.T) {
			s := src.run(t, false)
			wantWinner(t, s, src.mover)
		})
	}
}

// E6: a 2 that a 9 just returned is frozen for the turn now starting, so it
// doesn't open a pending win. Synthetic: P1 is already at 21 (as in
// TestWinTiming_MoverWinsFirstWhenBothOver) and its 9 bounces P2's 2♣ point.
func TestPendingWin_E6_FrozenTwoDoesNotCount(t *testing.T) {
	run := func(p2Hand []card.Card) GameState {
		s := pwBase(t, []card.Card{pwNineS, crTenD}, p2Hand)
		s.Players[P1].Points = append(s.Players[P1].Points, PointEntry{Card: pwSevC, Owner: P1})
		s.Players[P2].Points = append(s.Players[P2].Points, PointEntry{Card: crTwoC, Owner: P2})
		return mustApply(t, s, Move{Kind: MoveOneOff, Card: pwNineS, HandIndex: 0,
			Target: &Target{Owner: P2, Zone: ZonePoints, Index: 2}})
	}
	t.Run("OnlyTheFrozenTwo", func(t *testing.T) {
		s := run([]card.Card{crTenS})
		wantWinner(t, s, P1)
	})
	t.Run("AnUnfrozenTwoToo", func(t *testing.T) {
		s := run([]card.Card{pwTwoH, crTenS})
		wantPendingWin(t, s, P2)
		want := []Move{{Kind: MoveCounter, Card: pwTwoH, HandIndex: 0}, {Kind: MoveDecline}}
		if got := LegalMoves(s); !reflect.DeepEqual(got, want) {
			t.Errorf("LegalMoves = %+v, want only the unfrozen 2♥ and Decline", got)
		}
	})
}

// E6: the phase-5 legal moves are exactly one Counter per unfrozen 2, in
// hand order, then Decline; Apply refuses anything else.
func TestPendingWin_E6_LegalMoves(t *testing.T) {
	s := popWin(t, pwBase(t, []card.Card{pwTwoS, crTenD}, []card.Card{crTwoC, crTenS, pwTwoH}))
	wantPendingWin(t, s, P2)
	// P2's hand: 2♣, 10♠, 2♥, Q♥ (the opening draw).
	want := []Move{
		{Kind: MoveCounter, Card: crTwoC, HandIndex: 0},
		{Kind: MoveCounter, Card: pwTwoH, HandIndex: 2},
		{Kind: MoveDecline},
	}
	if got := LegalMoves(s); !reflect.DeepEqual(got, want) {
		t.Errorf("LegalMoves = %+v, want %+v", got, want)
	}
	for _, m := range []Move{
		{Kind: MoveDraw},
		{Kind: MovePlayPoint, Card: crTenS, HandIndex: 1},
		{Kind: MovePass},
	} {
		if _, err := Apply(s, m); err == nil {
			t.Errorf("Apply(%+v) in PhasePendingWin should be illegal", m)
		}
	}
}

// E6: Decline makes the mover the winner; Counter rewinds and starts the
// counterer's normal turn.
func TestPendingWin_E6_DeclineAndCounter(t *testing.T) {
	pending := popWin(t, pwBase(t, []card.Card{pwTwoS, crTenD}, []card.Card{crTwoC, crTenS}))
	wantPendingWin(t, pending, P2)

	t.Run("Decline", func(t *testing.T) {
		s := mustApply(t, pending, Move{Kind: MoveDecline})
		wantWinner(t, s, P1)
	})
	t.Run("Counter", func(t *testing.T) {
		s := mustCounter(t, pending, crTwoC)
		wantRestore(t, pending, s, P2, []card.Card{pwTwoS}, crTwoC)
		if PointTotal(s.Players[P1]) != 14 {
			t.Errorf("P1's win should be erased, total = %d", PointTotal(s.Players[P1]))
		}
		if _, ok := findMove(LegalMoves(s), func(m Move) bool { return m.Kind == MoveDraw }); !ok {
			t.Errorf("P2 should take a normal turn, LegalMoves=%+v", LegalMoves(s))
		}
		if _, ok := findMove(LegalMoves(s), func(m Move) bool { return m.Kind == MoveDecline }); ok {
			t.Errorf("no Decline in the counterer's normal turn")
		}
	})
}

// E6: a counter-back restores the winning position and brings the pending
// win back; countering it again rewinds to the end of the counterer's
// counter turn.
func TestPendingWin_E6_CounterBackBringsItBack(t *testing.T) {
	s := pwBase(t, []card.Card{pwTwoS, pwTwoD}, []card.Card{crTwoC, crTenS, pwTwoH})
	s = popWin(t, s)
	wantPendingWin(t, s, P2)
	s = mustCounter(t, s, crTwoC)
	s = mustApply(t, s, Move{Kind: MoveDraw})
	if s.Phase != PhaseNormal || s.Active != P1 || PointTotal(s.Players[P1]) != 14 {
		t.Fatalf("P1 should be to move at 14, phase=%v active=%v total=%d", s.Phase, s.Active, PointTotal(s.Players[P1]))
	}
	afterCounterTurn := s
	s = mustCounter(t, s, pwTwoD)
	wantPendingWin(t, s, P2)
	if PointTotal(s.Players[P1]) != 21 {
		t.Errorf("the counter-back should restore P1's 21, got %d", PointTotal(s.Players[P1]))
	}
	if !reflect.DeepEqual(s.LastTurn, TurnRecord{Counterable: true, Countered: []card.Card{pwTwoD}}) {
		t.Errorf("LastTurn = %+v, want P1's counter turn [2♦]", s.LastTurn)
	}
	if handIndexOf(s.Players[P2].Hand, crTwoC) >= 0 || handIndexOf(s.Scrap, crTwoC) < 0 {
		t.Errorf("P2's 2♣ should be scrapped by the counter-back")
	}
	// P2 counters again: back to the end of its own counter turn.
	s2 := mustCounter(t, s, pwTwoH)
	if !reflect.DeepEqual(s2.Players[P2].Hand, func() []card.Card {
		h := append([]card.Card(nil), afterCounterTurn.Players[P2].Hand...)
		return removeAt(h, handIndexOf(h, pwTwoH))
	}()) {
		t.Errorf("P2's hand should be the end of its counter turn minus 2♥, got %v", s2.Players[P2].Hand)
	}
	if s2.Phase != PhaseNormal || s2.Active != P2 || PointTotal(s2.Players[P1]) != 14 {
		t.Errorf("P2 should take a normal turn with P1 back at 14, phase=%v active=%v total=%d", s2.Phase, s2.Active, PointTotal(s2.Players[P1]))
	}
	if handIndexOf(s2.Scrap, pwTwoD) < 0 || handIndexOf(s2.Players[P1].Hand, pwTwoD) >= 0 {
		t.Errorf("P1's 2♦ should be scrapped")
	}
}

// E6: with both players at threshold after a counterable turn, the mover's
// claim comes first. Synthetic: P2 is already at 21 (one action can't take
// both players over; see TestWinTiming_MoverWinsFirstWhenBothOver).
func TestPendingWin_E6_BothAtThreshold_MoverFirst(t *testing.T) {
	// P2 has a King (threshold 14) and 9♦ added; after the pop it keeps
	// 5♦ + 9♦ = 14 while P1 reaches 21.
	t.Run("WithTwo", func(t *testing.T) {
		s := pwBase(t, []card.Card{pwTwoS, crTenD}, []card.Card{crTwoC, crTenS})
		s.Players[P2].Permanents = []card.Card{crKingC} // threshold 14
		s.Players[P2].Points = append(s.Players[P2].Points, PointEntry{Card: pc(card.Nine, card.Diamonds), Owner: P2})
		s = popWin(t, s)
		if !HasWon(s.Players[P1]) || !HasWon(s.Players[P2]) {
			t.Fatalf("setup: both should be at threshold, P1=%d P2=%d", PointTotal(s.Players[P1]), PointTotal(s.Players[P2]))
		}
		wantPendingWin(t, s, P2)
		// Declining still hands P1 the game: P2's own points never award it.
		wantWinner(t, mustApply(t, s, Move{Kind: MoveDecline}), P1)
	})
	t.Run("NoTwo", func(t *testing.T) {
		s := pwBase(t, []card.Card{pwTwoS, crTenD}, []card.Card{pwSevC, crTenS})
		s.Players[P2].Permanents = []card.Card{crKingC}
		s.Players[P2].Points = append(s.Players[P2].Points, PointEntry{Card: pc(card.Nine, card.Diamonds), Owner: P2})
		s = popWin(t, s)
		if !HasWon(s.Players[P2]) {
			t.Fatalf("setup: P2 should be at threshold too")
		}
		wantWinner(t, s, P1)
	})
}

// E6: the start-of-turn win is checked before the counter option. P1's 6
// strips its Jack off P2's 7♥, taking P2 to 21; P2 holds an unfrozen 2 and
// could counter the 6, but wins instead and is offered nothing.
func TestPendingWin_E6_TurnStartWinBeforeCounter(t *testing.T) {
	six := pc(card.Six, card.Spades)
	s := twoPlayerStart([]card.Card{six, crTenD}, []card.Card{crTwoC, crTenS}, []card.Card{crQH, crThrH})
	s.Players[P1].Points = []PointEntry{{Card: pwSevH, Owner: P2, JackStack: []card.Card{crJackS}, JackOwners: []PlayerID{P1}}}
	s.Players[P2].Points = []PointEntry{{Card: crTenH, Owner: P2}, {Card: pc(card.Four, card.Hearts), Owner: P2}}
	s.Active = P2
	s = mustApply(t, s, Move{Kind: MoveDraw})
	s = mustApply(t, s, Move{Kind: MoveOneOff, Card: six, HandIndex: 0})
	if !s.LastTurn.Counterable || s.TurnEnd[P2] == nil {
		t.Fatalf("setup: P2 should be able to counter the 6, last=%+v", s.LastTurn)
	}
	wantWinner(t, s, P2)
}

// E6: a win on a turn that can't be countered is immediate, whatever the
// opponent holds: no handover, no snapshot, no pending win.
func TestPendingWin_E6_NonCounterableWinIsImmediate(t *testing.T) {
	for _, p2Hand := range [][]card.Card{{crTwoC, crTenS}, {pwSevC, crTenS}} {
		before := pwBase(t, []card.Card{pwSevS, crTenD}, p2Hand)
		s := mustApply(t, before, Move{Kind: MovePlayPoint, Card: pwSevS, HandIndex: 0})
		wantWinner(t, s, P1)
		if s.TurnEnd != before.TurnEnd || !reflect.DeepEqual(s.LastTurn, before.LastTurn) {
			t.Errorf("an immediate win must not hand the turn over")
		}
	}
}

// E1 twins for a winning move: the accepted leak (design §4). After P1's
// winning counterable turn the twins differ, phase 5 with P2 deciding when P2
// holds a 2, game over with P1 the winner when it doesn't. Every non-winning
// Apply stays identical (TestCounterRewind_E1_TwinsSamePhaseAndActive).
func TestPendingWin_E1_WinningTwinsDiffer(t *testing.T) {
	a := popWin(t, pwBase(t, []card.Card{pwTwoS, crTenD}, []card.Card{crTwoC, crTenS}))
	b := popWin(t, pwBase(t, []card.Card{pwTwoS, crTenD}, []card.Card{pwSevC, crTenS}))
	if a.Phase != PhasePendingWin || a.Active != P2 {
		t.Errorf("with a 2: phase=%v active=%v, want PhasePendingWin, P2", a.Phase, a.Active)
	}
	if b.Phase != PhaseGameOver || b.Active != P1 || b.Winner == nil || *b.Winner != P1 {
		t.Errorf("without a 2: phase=%v active=%v winner=%v, want PhaseGameOver, P1 winning", b.Phase, b.Active, b.Winner)
	}
}

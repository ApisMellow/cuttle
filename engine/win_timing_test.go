package engine

import (
	"testing"

	"github.com/ApisMellow/cuttle/card"
)

// Win timing ruling: a player can reach their win threshold during the
// opponent's turn (the opponent pops their own Jack with a 2, or plays a 6,
// and a point card returns to the player). There is no off-turn win. The
// player wins at the start of their next turn, as soon as the turn passes to
// them and before they must make any move.
//
// A counter window is not a turn. The player answering a counter is Active,
// but their turn has not begun; it begins only once the chain resolves and
// the turn passes. (Since v0.3.0 new play never opens a counter window; the
// window tests below start from a legacy phase-1 state built directly.)

var (
	wtTwoH  = pc(card.Two, card.Hearts)
	wtTwoD  = pc(card.Two, card.Diamonds)
	wtTwoC  = pc(card.Two, card.Clubs)
	wtNine  = pc(card.Nine, card.Hearts)
	wtSix   = pc(card.Six, card.Diamonds)
	wtKingD = pc(card.King, card.Diamonds)
	wtKingH = pc(card.King, card.Hearts)
)

// popOwnJackState: P1 holds p1Hand and has P2's 9 under a P1 Jack. P2 has a
// 6 of its own and p2Perm permanents. P1 popping its Jack returns the 9 to
// P2, taking P2 to 15.
func popOwnJackState(p1Hand, p2Hand []card.Card, p2Perm []card.Card) GameState {
	s := twoPlayerStart(p1Hand, p2Hand, []card.Card{pc(card.Ace, card.Clubs)})
	s.Players[P1].Points = []PointEntry{{Card: wtNine, Owner: P2, JackStack: []card.Card{popJC}, JackOwners: []PlayerID{P1}}}
	s.Players[P2].Points = []PointEntry{{Card: wtSix, Owner: P2}}
	s.Players[P2].Permanents = p2Perm
	return s
}

var popOwnJackTarget = Target{Owner: P1, Zone: ZonePoints, Index: 0}

func mustApply(t *testing.T, s GameState, m Move) GameState {
	t.Helper()
	out, err := Apply(s, m)
	if err != nil {
		t.Fatalf("apply %+v: %v", m, err)
	}
	return out
}

func wantWinAtTurnStart(t *testing.T, s GameState, w PlayerID) {
	t.Helper()
	if s.Phase != PhaseGameOver || s.Winner == nil || *s.Winner != w {
		t.Fatalf("want %v to win (PhaseGameOver), got phase=%v winner=%v", w, s.Phase, s.Winner)
	}
	if s.Active != w {
		t.Errorf("winner %v should be the player whose turn began, Active=%v", w, s.Active)
	}
	if s.Pending != nil {
		t.Errorf("no pending one-off expected after game over, got %+v", s.Pending)
	}
	if LegalMoves(s) != nil {
		t.Errorf("winner must not have to move, LegalMoves=%+v", LegalMoves(s))
	}
}

func wantNoWin(t *testing.T, s GameState, phase Phase, active PlayerID) {
	t.Helper()
	if s.Winner != nil || s.Phase != phase || s.Active != active {
		t.Fatalf("want no winner, phase=%v, active=%v; got phase=%v winner=%v active=%v",
			phase, active, s.Phase, s.Winner, s.Active)
	}
}

// The pop resolves after P2 declines to counter. While the counter window is
// open P2 is Active but no win fires; once the 2 resolves the turn passes and
// P2 wins at the start of it.
func TestWinTiming_PopAfterDecline_OpponentWinsAtTurnStart(t *testing.T) {
	s := popOwnJackState([]card.Card{popTwo}, []card.Card{wtTwoH}, []card.Card{wtKingD})
	s1 := openLegacyWindow(t, s, Move{Kind: MoveOneOff, Card: popTwo, HandIndex: 0, Target: &popOwnJackTarget})
	wantNoWin(t, s1, PhaseAwaitingCounter, P2)

	s2 := mustApply(t, s1, Move{Kind: MoveDecline})
	if PointTotal(s2.Players[P2]) != 15 {
		t.Fatalf("P2 total = %d, want 15", PointTotal(s2.Players[P2]))
	}
	wantWinAtTurnStart(t, s2, P2)
}

// The pop 2 sits at the bottom of a counter chain. Every hand-off of the
// window flips Active, including to P2, without firing the win. An even chain
// resolves the pop, then the turn passes and P2 wins.
func TestWinTiming_PopResolvesThroughEvenChain_WinsOnlyAfterChain(t *testing.T) {
	s := popOwnJackState([]card.Card{popTwo, wtTwoD}, []card.Card{wtTwoH, wtTwoC}, []card.Card{wtKingD})
	s1 := openLegacyWindow(t, s, Move{Kind: MoveOneOff, Card: popTwo, HandIndex: 0, Target: &popOwnJackTarget})
	wantNoWin(t, s1, PhaseAwaitingCounter, P2)

	s2 := mustApply(t, s1, Move{Kind: MoveCounter, Card: wtTwoH, HandIndex: 0})
	wantNoWin(t, s2, PhaseAwaitingCounter, P1)

	s3 := mustApply(t, s2, Move{Kind: MoveCounter, Card: wtTwoD, HandIndex: 0})
	wantNoWin(t, s3, PhaseAwaitingCounter, P2)

	s4 := mustApply(t, s3, Move{Kind: MoveDecline})
	wantWinAtTurnStart(t, s4, P2)
}

// An odd chain cancels the pop: the 9 never moves, P2 stays at 6 and simply
// takes its turn.
func TestWinTiming_PopCancelledByOddChain_NoWin(t *testing.T) {
	s := popOwnJackState([]card.Card{popTwo}, []card.Card{wtTwoH}, []card.Card{wtKingD})
	s1 := openLegacyWindow(t, s, Move{Kind: MoveOneOff, Card: popTwo, HandIndex: 0, Target: &popOwnJackTarget})
	s2 := mustApply(t, s1, Move{Kind: MoveCounter, Card: wtTwoH, HandIndex: 0})
	wantNoWin(t, s2, PhaseNormal, P2)
	if PointTotal(s2.Players[P2]) != 6 {
		t.Errorf("P2 total = %d, want 6", PointTotal(s2.Players[P2]))
	}
}

// Synthetic state pinning that the check keys on the turn beginning, not on
// Active changing: P2 is already over threshold while P1 plays a 5. When P2 is
// handed the counter window it does not win; it wins only when the window
// closes and its turn begins.
func TestWinTiming_NoWinInsideCounterWindow(t *testing.T) {
	five := pc(card.Five, card.Clubs)
	s := twoPlayerStart([]card.Card{five}, []card.Card{wtTwoH}, nil)
	s.Players[P2].Points = []PointEntry{{Card: pc(card.Ten, card.Clubs), Owner: P2}, {Card: pc(card.Nine, card.Clubs), Owner: P2}}
	s.Players[P2].Permanents = []card.Card{wtKingD}

	s1 := openLegacyWindow(t, s, Move{Kind: MoveOneOff, Card: five, HandIndex: 0})
	wantNoWin(t, s1, PhaseAwaitingCounter, P2)

	s2 := mustApply(t, s1, Move{Kind: MoveCounter, Card: wtTwoH, HandIndex: 0})
	wantWinAtTurnStart(t, s2, P2)
}

// Threshold at turn start: Kings on the incoming player's side lower it, Kings
// on the other side don't count, and reaching the threshold exactly is a win.
func TestWinTiming_TurnStartThreshold(t *testing.T) {
	cases := []struct {
		name    string
		p2Own   []PointEntry
		p1Perm  []card.Card
		p2Perm  []card.Card
		wantWin bool
	}{
		{name: "NoKings_15_Under21_NoWin", p2Own: []PointEntry{{Card: wtSix, Owner: P2}}},
		{name: "OneKing_15_Over14_Wins", p2Own: []PointEntry{{Card: wtSix, Owner: P2}}, p2Perm: []card.Card{wtKingD}, wantWin: true},
		{name: "TwoKings_Exactly10_Wins", p2Own: []PointEntry{{Card: pc(card.Ace, card.Hearts), Owner: P2}}, p2Perm: []card.Card{wtKingD, wtKingH}, wantWin: true},
		{name: "TwoKings_9_Under10_NoWin", p2Perm: []card.Card{wtKingD, wtKingH}},
		{name: "OpponentsKingsDontCount_NoWin", p2Own: []PointEntry{{Card: wtSix, Owner: P2}}, p1Perm: []card.Card{wtKingD, wtKingH}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := popOwnJackState([]card.Card{popTwo}, nil, c.p2Perm)
			s.Players[P2].Points = c.p2Own
			s.Players[P1].Permanents = c.p1Perm
			m, ok := findMove(LegalMoves(s), func(m Move) bool {
				return m.Kind == MoveOneOff && m.Card == popTwo && m.Target != nil && *m.Target == popOwnJackTarget
			})
			if !ok {
				t.Fatalf("pop move not offered")
			}
			s1 := mustApply(t, s, m)
			if c.wantWin {
				wantWinAtTurnStart(t, s1, P2)
				return
			}
			wantNoWin(t, s1, PhaseNormal, P2)
			if _, ok := findMove(LegalMoves(s1), func(m Move) bool { return m.Kind == MoveDraw }); !ok {
				t.Errorf("P2 under threshold should take a normal turn, LegalMoves=%+v", LegalMoves(s1))
			}
		})
	}
}

// A 6 is the other known path: it strips every Jack and returns points to
// their original owners, which can take the non-mover to their threshold.
func TestWinTiming_SixReturnsPoints_OpponentWinsAtTurnStart(t *testing.T) {
	six := pc(card.Six, card.Clubs)
	s := twoPlayerStart([]card.Card{six}, nil, nil)
	s.Players[P1].Points = []PointEntry{{Card: pc(card.Seven, card.Hearts), Owner: P2, JackStack: []card.Card{popJC}, JackOwners: []PlayerID{P1}}}
	s.Players[P2].Points = []PointEntry{{Card: pc(card.Ten, card.Clubs), Owner: P2}, {Card: pc(card.Four, card.Clubs), Owner: P2}}

	s1 := mustApply(t, s, Move{Kind: MoveOneOff, Card: six, HandIndex: 0})
	if PointTotal(s1.Players[P2]) != 21 {
		t.Fatalf("P2 total = %d, want 21", PointTotal(s1.Players[P2]))
	}
	wantWinAtTurnStart(t, s1, P2)
}

// When one action takes both players over, the player who acted wins on their
// own action; the turn never passes.
func TestWinTiming_MoverWinsFirstWhenBothOver(t *testing.T) {
	six := pc(card.Six, card.Clubs)
	s := twoPlayerStart([]card.Card{six}, nil, nil)
	s.Players[P1].Points = []PointEntry{
		{Card: pc(card.Ten, card.Hearts), Owner: P1},
		{Card: pc(card.Four, card.Hearts), Owner: P1},
		{Card: pc(card.Seven, card.Hearts), Owner: P2, JackStack: []card.Card{popJC}, JackOwners: []PlayerID{P1}},
	}
	s.Players[P2].Points = []PointEntry{
		{Card: pc(card.Ten, card.Clubs), Owner: P2},
		{Card: pc(card.Four, card.Clubs), Owner: P2},
		{Card: pc(card.Seven, card.Clubs), Owner: P1, JackStack: []card.Card{popJD}, JackOwners: []PlayerID{P2}},
	}
	s1 := mustApply(t, s, Move{Kind: MoveOneOff, Card: six, HandIndex: 0})
	if s1.Phase != PhaseGameOver || s1.Winner == nil || *s1.Winner != P1 || s1.Active != P1 {
		t.Errorf("mover P1 should win on its own action, phase=%v winner=%v active=%v", s1.Phase, s1.Winner, s1.Active)
	}
}

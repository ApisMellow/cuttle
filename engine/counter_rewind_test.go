package engine

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ApisMellow/cuttle/card"
)

// v0.3.0 counter rewind (cuttle-web docs/counter-rewind-design.md §2, §3).
// A one-off takes effect at once. At the start of their turn the opponent
// may counter it with an unfrozen 2: the game goes back to TurnEnd[counterer],
// the countered cards and the counter 2 go to the scrap, and the counterer
// takes a normal turn. Test names carry the design's §8 case ids.

var (
	crTwoC  = pc(card.Two, card.Clubs) // P2's counter 2
	crTenS  = pc(card.Ten, card.Spades)
	crTenD  = pc(card.Ten, card.Diamonds) // P1 filler
	crSevH  = pc(card.Seven, card.Hearts) // P1 point
	crEigC  = pc(card.Eight, card.Clubs)  // P1 glasses
	crFivD  = pc(card.Five, card.Diamonds)
	crKingC = pc(card.King, card.Clubs)
	crQH    = pc(card.Queen, card.Hearts) // deck top: P2's opening draw
	crThrH  = pc(card.Three, card.Hearts)
	crFivC  = pc(card.Five, card.Clubs)
	crKingH = pc(card.King, card.Hearts)
	crNineC = pc(card.Nine, card.Clubs)
	crTenH  = pc(card.Ten, card.Hearts)
	crFourC = pc(card.Four, card.Clubs)
	crJackS = pc(card.Jack, card.Spades)
)

// rewindBase: P2 is to move and opens with a draw, so TurnEnd[P2] exists
// when P1 then plays its one-off. P1 holds oneOff and a filler 10; P2 holds
// p2Two (or a twin card) and a 10.
func rewindBase(oneOff, p2Two card.Card) GameState {
	s := twoPlayerStart([]card.Card{oneOff, crTenD}, []card.Card{p2Two, crTenS},
		[]card.Card{crQH, crThrH, crFivC, crKingH, crNineC})
	s.Players[P1].Points = []PointEntry{{Card: crSevH, Owner: P1}}
	s.Players[P1].Permanents = []card.Card{crEigC}
	s.Players[P2].Points = []PointEntry{{Card: crFivD, Owner: P2}}
	s.Players[P2].Permanents = []card.Card{crKingC}
	s.Scrap = []card.Card{crTenH, crFourC, crJackS}
	s.Active = P2
	return s
}

func counterMoves(s GameState) []Move {
	var out []Move
	for _, m := range LegalMoves(s) {
		if m.Kind == MoveCounter {
			out = append(out, m)
		}
	}
	return out
}

func mustCounter(t *testing.T, s GameState, two card.Card) GameState {
	t.Helper()
	m, ok := findMove(LegalMoves(s), func(m Move) bool { return m.Kind == MoveCounter && m.Card == two })
	if !ok {
		t.Fatalf("counter with %v not offered; LegalMoves=%+v", two, LegalMoves(s))
	}
	return mustApply(t, s, m)
}

// wantRestore asserts got is exactly snap plus the scrapped cards: each of
// fromOpp leaves the opponent's hand for the scrap, in order, then two
// leaves the counterer's hand on top. The counterer is active in PhaseNormal
// with no marks, no passes, and TurnEnd untouched.
func wantRestore(t *testing.T, before, got GameState, counterer PlayerID, fromOpp []card.Card, two card.Card) {
	t.Helper()
	snap := before.TurnEnd[counterer]
	if snap == nil {
		t.Fatalf("no TurnEnd[%v] to restore", counterer)
	}
	want := clone(*snap)
	opp := counterer.Other()
	for _, c := range fromOpp {
		i := handIndexOf(want.Players[opp].Hand, c)
		if i < 0 {
			t.Fatalf("%v not in P%d's hand at the snapshot", c, opp+1)
		}
		want.Players[opp].Hand = removeAt(want.Players[opp].Hand, i)
		want.Scrap = append(want.Scrap, c)
	}
	i := handIndexOf(want.Players[counterer].Hand, two)
	if i < 0 {
		t.Fatalf("counter %v not in P%d's hand at the snapshot", two, counterer+1)
	}
	want.Players[counterer].Hand = removeAt(want.Players[counterer].Hand, i)
	want.Scrap = append(want.Scrap, two)
	want.Players[P1].FrozenIDs = nil
	want.Players[P2].FrozenIDs = nil
	want.Active = counterer
	want.Phase = PhaseNormal
	want.PassesInARow = 0
	want.LastTurn = TurnRecord{}
	want.ThisTurn = TurnRecord{Counterable: true, Countered: []card.Card{two}}
	want.TurnEnd = before.TurnEnd
	if !reflect.DeepEqual(clone(want), clone(got)) {
		t.Errorf("restore mismatch\n want players=%+v\n  got players=%+v\n want deck=%v scrap=%v\n  got deck=%v scrap=%v\n want active=%v phase=%v last=%+v this=%+v\n  got active=%v phase=%v last=%+v this=%+v",
			want.Players, got.Players, want.Deck, want.Scrap, got.Deck, got.Scrap,
			want.Active, want.Phase, want.LastTurn, want.ThisTurn, got.Active, got.Phase, got.LastTurn, got.ThisTurn)
	}
	if got.TurnEnd != before.TurnEnd {
		t.Errorf("a restore must not touch TurnEnd")
	}
}

type rewindCase struct {
	name   string
	oneOff card.Card
	// wantPhase is P2's phase at the start of its turn (the zero value is
	// PhaseNormal; a 4 opens the owed-discard step).
	wantPhase Phase
	// p1Turn plays P1's turn from s (P1 active) and returns the state at
	// P2's turn start.
	p1Turn func(t *testing.T, s GameState) GameState
	// check makes case-specific assertions on the state after the counter.
	check func(t *testing.T, played, after GameState)
}

func oneOffTurn(m Move) func(t *testing.T, s GameState) GameState {
	return func(t *testing.T, s GameState) GameState {
		t.Helper()
		m.HandIndex = handIndexOf(s.Players[P1].Hand, m.Card)
		return mustApply(t, s, m)
	}
}

func sevenTurn(pick func(Move) bool) func(t *testing.T, s GameState) GameState {
	seven := pc(card.Seven, card.Spades)
	return func(t *testing.T, s GameState) GameState {
		t.Helper()
		s1 := mustApply(t, s, Move{Kind: MoveOneOff, Card: seven, HandIndex: handIndexOf(s.Players[P1].Hand, seven)})
		m, ok := findMove(LegalMoves(s1), pick)
		if !ok {
			t.Fatalf("7 pick not offered: %+v", LegalMoves(s1))
		}
		return mustApply(t, s1, m)
	}
}

func rewindCases() []rewindCase {
	p2Perm := Target{Owner: P2, Zone: ZonePermanents, Index: 0}
	p2Point := Target{Owner: P2, Zone: ZonePoints, Index: 0}
	deckAfterOpening := []card.Card{crThrH, crFivC, crKingH, crNineC}
	wantDeck := func(t *testing.T, after GameState) {
		t.Helper()
		if !reflect.DeepEqual(after.Deck, deckAfterOpening) {
			t.Errorf("deck = %v, want %v back in order", after.Deck, deckAfterOpening)
		}
	}
	return []rewindCase{
		{name: "Ace", oneOff: pc(card.Ace, card.Spades),
			p1Turn: oneOffTurn(Move{Kind: MoveOneOff, Card: pc(card.Ace, card.Spades)}),
			check: func(t *testing.T, played, after GameState) {
				if len(played.Players[P1].Points)+len(played.Players[P2].Points) != 0 {
					t.Fatalf("the Ace should have wiped the points before the counter")
				}
				if len(after.Players[P1].Points) != 1 || len(after.Players[P2].Points) != 1 {
					t.Errorf("points not restored: %+v / %+v", after.Players[P1].Points, after.Players[P2].Points)
				}
			}},
		{name: "TwoScrap", oneOff: pc(card.Two, card.Spades),
			p1Turn: oneOffTurn(Move{Kind: MoveOneOff, Card: pc(card.Two, card.Spades), Target: &p2Perm}),
			check: func(t *testing.T, played, after GameState) {
				if len(played.Players[P2].Permanents) != 0 {
					t.Fatalf("the 2 should have scrapped the King before the counter")
				}
				if !reflect.DeepEqual(after.Players[P2].Permanents, []card.Card{crKingC}) {
					t.Errorf("King not restored: %v", after.Players[P2].Permanents)
				}
			}},
		{name: "Three", oneOff: pc(card.Three, card.Spades),
			p1Turn: oneOffTurn(Move{Kind: MoveOneOff, Card: pc(card.Three, card.Spades), ScrapIndex: 1}),
			check: func(t *testing.T, played, after GameState) {
				if handIndexOf(played.Players[P1].Hand, crFourC) < 0 {
					t.Fatalf("the 3 should have taken 4♣ before the counter")
				}
				if after.Scrap[1] != crFourC || handIndexOf(after.Players[P1].Hand, crFourC) >= 0 {
					t.Errorf("4♣ should be back at scrap index 1: scrap=%v hand=%v", after.Scrap, after.Players[P1].Hand)
				}
			}},
		{name: "Four", oneOff: pc(card.Four, card.Spades), wantPhase: PhaseAwaitingDiscard,
			// P2's turn opens on the owed discard; the counter erases the debt.
			p1Turn: oneOffTurn(Move{Kind: MoveOneOff, Card: pc(card.Four, card.Spades)}),
			check: func(t *testing.T, played, after GameState) {
				if played.Owed != [2]int{0, 2} || len(played.Players[P2].Hand) != 3 {
					t.Fatalf("P2 should owe 2 with a full hand, owed=%v hand=%v", played.Owed, played.Players[P2].Hand)
				}
				if after.Owed != [2]int{} || !reflect.DeepEqual(after.Players[P2].Hand, []card.Card{crTenS, crQH}) {
					t.Errorf("debt should disappear and P2 keep 10♠ Q♥: owed=%v hand=%v", after.Owed, after.Players[P2].Hand)
				}
			}},
		{name: "Five", oneOff: pc(card.Five, card.Spades),
			p1Turn: oneOffTurn(Move{Kind: MoveOneOff, Card: pc(card.Five, card.Spades)}),
			check: func(t *testing.T, played, after GameState) {
				if len(played.Deck) != 2 {
					t.Fatalf("the 5 should have drawn two, deck=%v", played.Deck)
				}
				wantDeck(t, after)
			}},
		{name: "Six", oneOff: pc(card.Six, card.Spades),
			p1Turn: oneOffTurn(Move{Kind: MoveOneOff, Card: pc(card.Six, card.Spades)}),
			check: func(t *testing.T, played, after GameState) {
				if len(played.Players[P1].Permanents)+len(played.Players[P2].Permanents) != 0 {
					t.Fatalf("the 6 should have wiped the permanents")
				}
				if len(after.Players[P1].Permanents) != 1 || len(after.Players[P2].Permanents) != 1 {
					t.Errorf("permanents not restored")
				}
			}},
		{name: "SevenPickPoint", oneOff: pc(card.Seven, card.Spades),
			p1Turn: sevenTurn(func(m Move) bool {
				return m.Kind == MoveSevenPick && m.Card == crThrH && m.SubMove.Kind == MovePlayPoint
			}),
			check: func(t *testing.T, played, after GameState) {
				if played.Deck[0] != crFivC {
					t.Fatalf("unchosen 5♣ should be on top before the counter, deck=%v", played.Deck)
				}
				wantDeck(t, after)
			}},
		{name: "SevenPickOneOff", oneOff: pc(card.Seven, card.Spades),
			// The picked 5♣ came from the deck: it is not a countered card
			// and goes back to the deck, not the scrap.
			p1Turn: sevenTurn(func(m Move) bool {
				return m.Kind == MoveSevenPick && m.Card == crFivC && m.SubMove.Kind == MoveOneOff
			}),
			check: func(t *testing.T, played, after GameState) {
				if !reflect.DeepEqual(played.LastTurn.Countered, []card.Card{pc(card.Seven, card.Spades)}) {
					t.Errorf("countered = %v, want only the 7", played.LastTurn.Countered)
				}
				wantDeck(t, after)
				for _, c := range after.Scrap {
					if c == crFivC {
						t.Errorf("picked 5♣ must not reach the scrap")
					}
				}
			}},
		{name: "Nine", oneOff: pc(card.Nine, card.Spades),
			p1Turn: oneOffTurn(Move{Kind: MoveOneOff, Card: pc(card.Nine, card.Spades), Target: &p2Point}),
			check: func(t *testing.T, played, after GameState) {
				i := handIndexOf(played.Players[P2].Hand, crFivD)
				if i < 0 || !played.Players[P2].FrozenIDs[i] {
					t.Fatalf("the 9 should have returned 5♦ frozen, hand=%v marks=%v", played.Players[P2].Hand, played.Players[P2].FrozenIDs)
				}
				if len(after.Players[P2].Points) != 1 || after.Players[P2].Points[0].Card != crFivD {
					t.Errorf("5♦ should be back on the board: %+v", after.Players[P2].Points)
				}
				if handIndexOf(after.Players[P2].Hand, crFivD) >= 0 || after.Players[P2].FrozenIDs != nil {
					t.Errorf("5♦ should have left the hand and lost its freeze")
				}
			}},
	}
}

// E3: exact restore for each one-off rank.
func TestCounterRewind_E3_ExactRestorePerRank(t *testing.T) {
	for _, c := range rewindCases() {
		t.Run(c.name, func(t *testing.T) {
			s0 := mustApply(t, rewindBase(c.oneOff, crTwoC), Move{Kind: MoveDraw})
			played := c.p1Turn(t, s0)
			if played.Phase != c.wantPhase || played.Active != P2 {
				t.Fatalf("P1's turn should end with P2 to move, phase=%v (want %v) active=%v", played.Phase, c.wantPhase, played.Active)
			}
			if !played.LastTurn.Counterable {
				t.Fatalf("P1's one-off turn should be counterable")
			}
			after := mustCounter(t, played, crTwoC)
			wantRestore(t, played, after, P2, []card.Card{c.oneOff}, crTwoC)
			c.check(t, played, after)
		})
	}
}

// E1 (light): a non-winning one-off gives the same Phase and Active whether
// or not the opponent holds a 2 (twins differ only by 2♣ vs 7♣ in P2's hand).
func TestCounterRewind_E1_TwinsSamePhaseAndActive(t *testing.T) {
	twin := pc(card.Seven, card.Clubs)
	for _, c := range rewindCases() {
		t.Run(c.name, func(t *testing.T) {
			a := mustApply(t, rewindBase(c.oneOff, crTwoC), Move{Kind: MoveDraw})
			b := mustApply(t, rewindBase(c.oneOff, twin), Move{Kind: MoveDraw})
			// The one-off's own Apply (phase 2 for a 7; a 4 ends the turn
			// into P2's owed discard in both twins).
			m := Move{Kind: MoveOneOff, Card: c.oneOff, HandIndex: 0}
			if lm, ok := findMove(LegalMoves(a), func(x Move) bool { return x.Kind == MoveOneOff && x.Card == c.oneOff }); ok {
				m = lm
			}
			a1, b1 := mustApply(t, a, m), mustApply(t, b, m)
			if a1.Phase != b1.Phase || a1.Active != b1.Active {
				t.Errorf("after the one-off: with 2 phase=%v active=%v, without phase=%v active=%v", a1.Phase, a1.Active, b1.Phase, b1.Active)
			}
			a2, b2 := c.p1Turn(t, a), c.p1Turn(t, b)
			if a2.Phase != b2.Phase || a2.Active != b2.Active {
				t.Errorf("after the turn: with 2 phase=%v active=%v, without phase=%v active=%v", a2.Phase, a2.Active, b2.Phase, b2.Active)
			}
		})
	}
	t.Run("CounterTurn", func(t *testing.T) {
		// P2 counters P1's Ace and plays 2♥ as a one-off on P1's glasses;
		// the twins differ by whether P1 still holds a 2 (2♦ vs 9♥).
		ace := pc(card.Ace, card.Spades)
		run := func(p1Extra card.Card) GameState {
			s := rewindBase(ace, crTwoC)
			s.Players[P1].Hand = []card.Card{ace, p1Extra}
			s.Players[P2].Hand = []card.Card{crTwoC, pc(card.Two, card.Hearts)}
			s = mustApply(t, s, Move{Kind: MoveDraw})
			s = mustApply(t, s, Move{Kind: MoveOneOff, Card: ace, HandIndex: 0})
			s = mustCounter(t, s, crTwoC)
			tgt := Target{Owner: P1, Zone: ZonePermanents, Index: 0}
			return mustApply(t, s, Move{Kind: MoveOneOff, Card: pc(card.Two, card.Hearts),
				HandIndex: handIndexOf(s.Players[P2].Hand, pc(card.Two, card.Hearts)), Target: &tgt})
		}
		a, b := run(pc(card.Two, card.Diamonds)), run(pc(card.Nine, card.Hearts))
		if a.Phase != b.Phase || a.Active != b.Active || a.Phase != PhaseNormal || a.Active != P1 {
			t.Errorf("with 2 phase=%v active=%v, without phase=%v active=%v; want PhaseNormal, P1", a.Phase, a.Active, b.Phase, b.Active)
		}
	})
}

// E2: the counter is offered only as the first move after a counterable
// turn, only for an unfrozen 2, never on the first turn, and is appended
// after every existing move.
func TestCounterRewind_E2_WhenOffered(t *testing.T) {
	ace := pc(card.Ace, card.Spades)

	t.Run("AfterCounterableTurn_AppendedLast", func(t *testing.T) {
		s := mustApply(t, rewindBase(ace, crTwoC), Move{Kind: MoveDraw})
		s = mustApply(t, s, Move{Kind: MoveOneOff, Card: ace, HandIndex: 0})
		moves := LegalMoves(s)
		plain := s
		plain.LastTurn = TurnRecord{}
		base := LegalMoves(plain)
		if len(moves) != len(base)+1 {
			t.Fatalf("want exactly one counter appended: %d moves vs %d without", len(moves), len(base))
		}
		if !reflect.DeepEqual(moves[:len(base)], base) {
			t.Errorf("existing moves must keep their positions")
		}
		last := moves[len(moves)-1]
		if last.Kind != MoveCounter || last.Card != crTwoC || s.Players[P2].Hand[last.HandIndex] != crTwoC {
			t.Errorf("last move = %+v, want counter with 2♣", last)
		}
	})

	t.Run("NotAfterNonCounterableTurn", func(t *testing.T) {
		s := mustApply(t, rewindBase(ace, crTwoC), Move{Kind: MoveDraw})
		s = mustApply(t, s, Move{Kind: MovePlayPoint, Card: crTenD, HandIndex: 1})
		if cm := counterMoves(s); len(cm) != 0 {
			t.Errorf("no counter after a point play, got %+v", cm)
		}
		if _, err := Apply(s, Move{Kind: MoveCounter, Card: crTwoC, HandIndex: 0}); err == nil {
			t.Errorf("Apply must reject the counter")
		}
	})

	t.Run("NeverOnFirstTurn_OpeningCounterable", func(t *testing.T) {
		// The game's first turn: nothing to counter.
		s := twoPlayerStart([]card.Card{crTwoC, ace}, []card.Card{pc(card.Two, card.Hearts)}, []card.Card{crQH})
		if cm := counterMoves(s); len(cm) != 0 {
			t.Errorf("no counter on the game's first turn, got %+v", cm)
		}
		// Part B (ruled 2026-10-25): P2's first turn may counter P1's
		// opening one-off; the rewind point is the dealt position.
		s1 := mustApply(t, s, Move{Kind: MoveOneOff, Card: ace, HandIndex: 1})
		if !s1.LastTurn.Counterable || s1.TurnEnd[P2] == nil {
			t.Fatalf("setup: last=%+v turnEnd[P2]=%v", s1.LastTurn, s1.TurnEnd[P2])
		}
		cm := counterMoves(s1)
		if len(cm) != 1 || cm[0].Card != pc(card.Two, card.Hearts) {
			t.Errorf("P2 should be offered 2♥ against the opening one-off, got %+v", cm)
		}
	})

	t.Run("OnlyUnfrozenTwos", func(t *testing.T) {
		s := mustApply(t, rewindBase(ace, crTwoC), Move{Kind: MoveDraw})
		s = mustApply(t, s, Move{Kind: MoveOneOff, Card: ace, HandIndex: 0})
		s.Players[P2].Hand = append(s.Players[P2].Hand, pc(card.Two, card.Hearts))
		s.Players[P2].FrozenIDs = map[int]bool{0: true}
		cm := counterMoves(s)
		if len(cm) != 1 || cm[0].Card != pc(card.Two, card.Hearts) {
			t.Errorf("only the unfrozen 2♥ may counter, got %+v", cm)
		}
		if _, err := Apply(s, Move{Kind: MoveCounter, Card: crTwoC, HandIndex: 0}); err == nil {
			t.Errorf("Apply must reject the frozen 2")
		}
	})

	t.Run("OtherFirstMoveRemovesIt", func(t *testing.T) {
		s := rewindBase(ace, crTwoC)
		s.Players[P2].Hand = append(s.Players[P2].Hand, pc(card.Two, card.Hearts))
		s = mustApply(t, s, Move{Kind: MoveDraw})
		s = mustApply(t, s, Move{Kind: MoveOneOff, Card: ace, HandIndex: 0})
		// After a counter the turn continues, and the second 2 is not offered.
		s1 := mustCounter(t, s, crTwoC)
		if cm := counterMoves(s1); len(cm) != 0 {
			t.Errorf("no second counter in one turn, got %+v", cm)
		}
		if _, err := Apply(s1, Move{Kind: MoveCounter, Card: pc(card.Two, card.Hearts),
			HandIndex: handIndexOf(s1.Players[P2].Hand, pc(card.Two, card.Hearts))}); err == nil {
			t.Errorf("Apply must reject a second counter")
		}
		// Any other first move lets the turn stand: P2 draws, the Ace's wipe
		// stays, and P2's draw turn is not counterable.
		s2 := mustApply(t, s, Move{Kind: MoveDraw})
		if s2.Active != P1 || s2.LastTurn.Counterable {
			t.Errorf("P2's draw should end a non-counterable turn, active=%v last=%+v", s2.Active, s2.LastTurn)
		}
		if len(s2.Players[P1].Points)+len(s2.Players[P2].Points) != 0 {
			t.Errorf("the Ace's effect should stand")
		}
	})
}

// E4: chains of 2, 3 and 4 counters.
func TestCounterRewind_E4_Chains(t *testing.T) {
	ace := pc(card.Ace, card.Spades)
	p1Two := pc(card.Two, card.Diamonds)
	p1Two2 := pc(card.Two, card.Spades)
	p2Two2 := pc(card.Two, card.Hearts)
	s := rewindBase(ace, crTwoC)
	s.Players[P1].Hand = []card.Card{ace, p1Two, p1Two2}
	s.Players[P2].Hand = []card.Card{crTwoC, p2Two2, crTenS}
	s = mustApply(t, s, Move{Kind: MoveDraw}) // P2 draws Q♥
	afterAce := mustApply(t, s, Move{Kind: MoveOneOff, Card: ace, HandIndex: 0})

	// Counter 1: P2 erases the Ace, then scuttles P1's 7♥ with 10♠.
	c1 := mustCounter(t, afterAce, crTwoC)
	wantRestore(t, afterAce, c1, P2, []card.Card{ace}, crTwoC)
	t1 := mustApply(t, c1, Move{Kind: MoveScuttle, Card: crTenS, HandIndex: handIndexOf(c1.Players[P2].Hand, crTenS),
		Target: &Target{Owner: P1, Zone: ZonePoints, Index: 0}})
	if !t1.LastTurn.Counterable || !reflect.DeepEqual(t1.LastTurn.Countered, []card.Card{crTwoC}) {
		t.Fatalf("counter turn record = %+v, want counterable with [2♣]", t1.LastTurn)
	}

	// Counter 2: P1 counters back to the position just after its Ace; the
	// scuttle is erased with the counter.
	c2 := mustCounter(t, t1, p1Two)
	wantRestore(t, t1, c2, P1, []card.Card{crTwoC}, p1Two)
	if len(c2.Players[P1].Points)+len(c2.Players[P2].Points) != 0 {
		t.Errorf("the Ace's wipe should be back, points=%+v / %+v", c2.Players[P1].Points, c2.Players[P2].Points)
	}
	if handIndexOf(c2.Players[P2].Hand, crTenS) < 0 {
		t.Errorf("10♠ should be back in P2's hand")
	}
	t2 := mustApply(t, c2, Move{Kind: MoveDraw})

	// Counter 3: P2 erases P1's counter-back turn.
	c3 := mustCounter(t, t2, p2Two2)
	wantRestore(t, t2, c3, P2, []card.Card{p1Two}, p2Two2)
	t3 := mustApply(t, c3, Move{Kind: MoveDraw})

	// Counter 4: P1 erases that, back to the end of its counter-back turn.
	c4 := mustCounter(t, t3, p1Two2)
	wantRestore(t, t3, c4, P1, []card.Card{p2Two2}, p1Two2)
	for _, two := range []card.Card{crTwoC, p1Two, p2Two2, p1Two2} {
		if handIndexOf(c4.Scrap, two) < 0 {
			t.Errorf("%v should be in the scrap after four counters, scrap=%v", two, c4.Scrap)
		}
	}
}

// E4: the design §2.2 example — a countered card that is not in its player's
// hand in the restored position stays where that position has it.
func TestCounterRewind_E4_ThreeDiamondsExample(t *testing.T) {
	fiveH := pc(card.Five, card.Hearts)
	threeD := pc(card.Three, card.Diamonds)
	aTwo := pc(card.Two, card.Diamonds)
	bTwo1, bTwo2 := pc(card.Two, card.Clubs), pc(card.Two, card.Hearts)
	s := twoPlayerStart([]card.Card{fiveH, aTwo}, []card.Card{bTwo1, bTwo2},
		[]card.Card{crQH, threeD, crKingH, crNineC, crFivC})
	s.Scrap = []card.Card{crTenH}
	s.Active = P2
	s = mustApply(t, s, Move{Kind: MoveDraw}) // Blake draws Q♥

	// 1. Alice plays 5♥ and draws 3♦ (and K♥).
	s = mustApply(t, s, Move{Kind: MoveOneOff, Card: fiveH, HandIndex: 0})
	if handIndexOf(s.Players[P1].Hand, threeD) < 0 {
		t.Fatalf("Alice should have drawn 3♦")
	}
	// 2. Blake counters; 3♦ goes back on top, and he draws it.
	s = mustCounter(t, s, bTwo1)
	if s.Deck[0] != threeD {
		t.Fatalf("3♦ should be back on top, deck=%v", s.Deck)
	}
	s = mustApply(t, s, Move{Kind: MoveDraw})
	if handIndexOf(s.Players[P2].Hand, threeD) < 0 {
		t.Fatalf("Blake should have drawn 3♦")
	}
	// 3. Alice counters back; 3♦ is in her hand again, and she plays it.
	s = mustCounter(t, s, aTwo)
	if handIndexOf(s.Players[P1].Hand, threeD) < 0 || handIndexOf(s.Players[P2].Hand, threeD) >= 0 {
		t.Fatalf("3♦ should be back in Alice's hand")
	}
	s = mustApply(t, s, Move{Kind: MoveOneOff, Card: threeD, HandIndex: handIndexOf(s.Players[P1].Hand, threeD), ScrapIndex: 0})
	// 4. Blake counters that turn. In the restored position 3♦ is in Blake's
	// hand, so it stays there; only Alice's 2 and Blake's 2 are scrapped.
	before := s
	s = mustCounter(t, s, bTwo2)
	wantRestore(t, before, s, P2, []card.Card{aTwo}, bTwo2)
	if handIndexOf(s.Players[P2].Hand, threeD) < 0 || handIndexOf(s.Scrap, threeD) >= 0 {
		t.Errorf("3♦ should stay in Blake's hand: hand=%v scrap=%v", s.Players[P2].Hand, s.Scrap)
	}
}

// E5: the reviewer's repro, rewritten for v0.3.0. Alice's 9 bounces Blake's
// 2-point. The 2 is frozen during Blake's next turn, so he can't counter the
// 9 with it, and it is free on the turn after (v0.2.x left the mark on
// through Alice's turn, blocking a counter of her next one-off).
func TestStaleFreezeBlocksCounter(t *testing.T) {
	nine := pc(card.Nine, card.Spades)
	ace := pc(card.Ace, card.Spades)
	s := twoPlayerStart([]card.Card{nine, ace}, []card.Card{crTenS}, []card.Card{crQH, crThrH, crKingH})
	s.Players[P2].Points = []PointEntry{{Card: crTwoC, Owner: P2}}
	s.Active = P2
	s = mustApply(t, s, Move{Kind: MoveDraw}) // Blake

	s = mustApply(t, s, Move{Kind: MoveOneOff, Card: nine, HandIndex: 0,
		Target: &Target{Owner: P2, Zone: ZonePoints, Index: 0}}) // Alice
	i := handIndexOf(s.Players[P2].Hand, crTwoC)
	if i < 0 || !s.Players[P2].FrozenIDs[i] {
		t.Fatalf("2♣ should be back in Blake's hand, frozen: hand=%v marks=%v", s.Players[P2].Hand, s.Players[P2].FrozenIDs)
	}
	if cm := counterMoves(s); len(cm) != 0 {
		t.Errorf("the frozen 2 must not counter the 9, got %+v", cm)
	}
	s = mustApply(t, s, Move{Kind: MoveDraw}) // Blake
	if s.Players[P2].FrozenIDs != nil {
		t.Errorf("the mark must end with Blake's frozen turn, marks=%v", s.Players[P2].FrozenIDs)
	}

	s = mustApply(t, s, Move{Kind: MoveOneOff, Card: ace, HandIndex: 0}) // Alice
	before := s
	s = mustCounter(t, s, crTwoC) // Blake, with the formerly frozen 2
	wantRestore(t, before, s, P2, []card.Card{ace}, crTwoC)
}

// E5: endTurn clears both players' marks, and so does a counter.
func TestCounterRewind_E5_FreezeClears(t *testing.T) {
	t.Run("EndTurnClearsBothSides", func(t *testing.T) {
		s := twoPlayerStart([]card.Card{crTenD, crKingH}, []card.Card{crTenS}, []card.Card{crQH})
		s.Players[P1].FrozenIDs = map[int]bool{1: true}
		s.Players[P2].FrozenIDs = map[int]bool{0: true}
		s1 := mustApply(t, s, Move{Kind: MovePlayPoint, Card: crTenD, HandIndex: 0})
		if s1.Players[P1].FrozenIDs != nil || s1.Players[P2].FrozenIDs != nil {
			t.Errorf("both sides must be cleared: P1=%v P2=%v", s1.Players[P1].FrozenIDs, s1.Players[P2].FrozenIDs)
		}
		snap := s1.TurnEnd[P1]
		if snap == nil || snap.Players[P1].FrozenIDs != nil || snap.Players[P2].FrozenIDs != nil {
			t.Errorf("the snapshot is taken after the clear")
		}
	})
	t.Run("CounterClearsAllMarks", func(t *testing.T) {
		// Counter P1's 9: the returned 5♦ was frozen; after the counter no
		// mark remains anywhere.
		var nineCase rewindCase
		for _, c := range rewindCases() {
			if c.name == "Nine" {
				nineCase = c
			}
		}
		s0 := mustApply(t, rewindBase(nineCase.oneOff, crTwoC), Move{Kind: MoveDraw})
		played := nineCase.p1Turn(t, s0)
		played.Players[P1].FrozenIDs = map[int]bool{0: true} // a stray mark too
		after := mustCounter(t, played, crTwoC)
		if after.Players[P1].FrozenIDs != nil || after.Players[P2].FrozenIDs != nil {
			t.Errorf("a counter clears every mark: P1=%v P2=%v", after.Players[P1].FrozenIDs, after.Players[P2].FrozenIDs)
		}
	})
}

// E8: PassesInARow is 0 after a counter; JSON round-trips a state carrying
// TurnEnd and LastTurn; depth stays at one through a chain.
func TestCounterRewind_E8_StateHygiene(t *testing.T) {
	ace := pc(card.Ace, card.Spades)
	p1Two := pc(card.Two, card.Diamonds)
	s := rewindBase(ace, crTwoC)
	s.Players[P1].Hand = []card.Card{ace, p1Two}
	s = mustApply(t, s, Move{Kind: MoveDraw})
	s = mustApply(t, s, Move{Kind: MoveOneOff, Card: ace, HandIndex: 0})

	t.Run("PassesZero", func(t *testing.T) {
		p := s
		p.PassesInARow = 2
		snap := clone(*s.TurnEnd[P2])
		snap.PassesInARow = 1
		p.TurnEnd[P2] = &snap
		after := mustCounter(t, p, crTwoC)
		if after.PassesInARow != 0 {
			t.Errorf("PassesInARow = %d, want 0", after.PassesInARow)
		}
	})

	c1 := mustCounter(t, s, crTwoC)
	t1 := mustApply(t, c1, Move{Kind: MoveDraw})
	c2 := mustCounter(t, t1, p1Two)

	t.Run("DepthOne", func(t *testing.T) {
		for _, st := range []GameState{s, c1, t1, c2} {
			for p, snap := range st.TurnEnd {
				if snap != nil && (snap.TurnEnd[P1] != nil || snap.TurnEnd[P2] != nil) {
					t.Errorf("TurnEnd[%d] carries its own snapshots", p)
				}
			}
		}
	})

	t.Run("JSONRoundTrip", func(t *testing.T) {
		for _, st := range []GameState{t1, c2} {
			if st.TurnEnd[P1] == nil || st.TurnEnd[P2] == nil {
				t.Fatalf("setup: want both snapshots")
			}
			b, err := json.Marshal(st)
			if err != nil {
				t.Fatal(err)
			}
			var back GameState
			if err := json.Unmarshal(b, &back); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(clone(st), clone(back)) {
				t.Errorf("round trip changed the state:\n%s", b)
			}
		}
		if !t1.LastTurn.Counterable || len(t1.LastTurn.Countered) != 1 {
			t.Errorf("setup: t1 should carry a counterable LastTurn, got %+v", t1.LastTurn)
		}
	})
}

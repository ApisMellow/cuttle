package engine

import (
	"testing"

	"github.com/ApisMellow/cuttle/card"
)

// Jack-pop ruling (2 as a one-off targeting a Jack on a point stack):
//
//   - The 2 removes only the TOP Jack of the stack; the popped Jack goes to
//     the scrap pile.
//   - Control passes to the owner of the next Jack down, or to the point
//     card's original owner when no Jacks remain.
//   - The PointEntry physically moves into the new controller's Points slice,
//     so PointTotal follows it.
//   - Buried Jacks cannot be targeted.
//   - The player who played the 2 wins if the pop takes them to their
//     threshold. If the pop instead takes the opponent there, the opponent
//     does not win off-turn; they win at the start of their next turn
//     (win_timing_test.go).

var (
	popTwo = card.Card{Rank: card.Two, Suit: card.Spades}
	popJC  = card.Card{Rank: card.Jack, Suit: card.Clubs}
	popJD  = card.Card{Rank: card.Jack, Suit: card.Diamonds}
	popJH  = card.Card{Rank: card.Jack, Suit: card.Hearts}
	popJS  = card.Card{Rank: card.Jack, Suit: card.Spades}
	popKC  = card.Card{Rank: card.King, Suit: card.Clubs}
)

func pc(r card.Rank, s card.Suit) card.Card { return card.Card{Rank: r, Suit: s} }

// findEntry locates the PointEntry whose underlying card is c. It fails the
// test unless the entry exists exactly once on the whole field.
func findEntry(t *testing.T, s GameState, c card.Card) (PlayerID, PointEntry) {
	t.Helper()
	var side PlayerID
	var got PointEntry
	n := 0
	for pl := 0; pl < 2; pl++ {
		for _, pe := range s.Players[pl].Points {
			if pe.Card == c {
				side, got = PlayerID(pl), pe
				n++
			}
		}
	}
	if n != 1 {
		t.Fatalf("expected point %v exactly once on the field, found %d (P1=%+v P2=%+v)",
			c, n, s.Players[P1].Points, s.Players[P2].Points)
	}
	return side, got
}

func scrapHas(s GameState, c card.Card) bool {
	for _, x := range s.Scrap {
		if x == c {
			return true
		}
	}
	return false
}

func TestTwoPopsTopJack(t *testing.T) {
	type tc struct {
		name   string
		actor  PlayerID
		p1     []PointEntry
		p2     []PointEntry
		p1Perm []card.Card
		p2Perm []card.Card
		target Target
		// expectations
		point      card.Card // underlying card of the targeted entry
		wantSide   PlayerID  // Points slice the entry must end up in
		wantStack  []card.Card
		wantOwners []PlayerID
		popped     card.Card
		wantP1     int
		wantP2     int
		wantWinner *PlayerID
	}
	p1, p2 := P1, P2
	seven := pc(card.Seven, card.Diamonds)
	five := pc(card.Five, card.Hearts)
	ten := pc(card.Ten, card.Spades)
	nine := pc(card.Nine, card.Hearts)

	cases := []tc{
		{
			name:  "OneJackPopped_ControlReturnsToOriginalOwner",
			actor: P1,
			p1:    []PointEntry{{Card: pc(card.Nine, card.Diamonds), Owner: P1}},
			p2: []PointEntry{
				{Card: five, Owner: P1, JackStack: []card.Card{popJC}, JackOwners: []PlayerID{P2}},
				{Card: pc(card.Three, card.Clubs), Owner: P2},
			},
			target:    Target{Owner: P2, Zone: ZonePoints, Index: 0},
			point:     five,
			wantSide:  P1,
			wantStack: nil, wantOwners: nil,
			popped: popJC,
			wantP1: 14, wantP2: 3,
		},
		{
			// Owner P2; Jacks P1, P2, P1 → entry sits on P1. P2 pops the
			// top (P1's) Jack; next Jack down is P2's, so the 7 moves to P2.
			name:  "ThreeAlternatingJacks_TopPopped_ControlToNextJackOwner",
			actor: P2,
			p1: []PointEntry{
				{Card: pc(card.Four, card.Clubs), Owner: P1},
				{Card: seven, Owner: P2, JackStack: []card.Card{popJC, popJH, popJS}, JackOwners: []PlayerID{P1, P2, P1}},
			},
			p2:         []PointEntry{{Card: pc(card.Six, card.Hearts), Owner: P2}},
			target:     Target{Owner: P1, Zone: ZonePoints, Index: 1},
			point:      seven,
			wantSide:   P2,
			wantStack:  []card.Card{popJC, popJH},
			wantOwners: []PlayerID{P1, P2},
			popped:     popJS,
			wantP1:     4, wantP2: 13,
		},
		{
			// Mirror: Owner P1; Jacks P2, P1, P2 → entry sits on P2. P1 pops
			// the top Jack; next Jack down is P1's, so the 10 moves to P1.
			name:       "ThreeAlternatingJacks_Mirror_ControlToNextJackOwner",
			actor:      P1,
			p2:         []PointEntry{{Card: ten, Owner: P1, JackStack: []card.Card{popJD, popJC, popJH}, JackOwners: []PlayerID{P2, P1, P2}}},
			target:     Target{Owner: P2, Zone: ZonePoints, Index: 0},
			point:      ten,
			wantSide:   P1,
			wantStack:  []card.Card{popJD, popJC},
			wantOwners: []PlayerID{P2, P1},
			popped:     popJH,
			wantP1:     10, wantP2: 0,
		},
		{
			// Two-Jack stack: popping the top leaves the bottom Jack, whose
			// owner (P1) is also the actor. P1 has a King (threshold 14):
			// 8 + 7 = 15 → P1 wins on the 2's resolution.
			name:       "PopMovesPointToActor_CrossesThreshold_Wins",
			actor:      P1,
			p1:         []PointEntry{{Card: pc(card.Eight, card.Clubs), Owner: P1}},
			p1Perm:     []card.Card{popKC},
			p2:         []PointEntry{{Card: seven, Owner: P2, JackStack: []card.Card{popJC, popJH}, JackOwners: []PlayerID{P1, P2}}},
			target:     Target{Owner: P2, Zone: ZonePoints, Index: 0},
			point:      seven,
			wantSide:   P1,
			wantStack:  []card.Card{popJC},
			wantOwners: []PlayerID{P1},
			popped:     popJH,
			wantP1:     15, wantP2: 0,
			wantWinner: &p1,
		},
		{
			// Same move without the King: 15 < 21, no win, turn passes.
			name:       "PopMovesPointToActor_BelowThreshold_NoWin",
			actor:      P1,
			p1:         []PointEntry{{Card: pc(card.Eight, card.Clubs), Owner: P1}},
			p2:         []PointEntry{{Card: seven, Owner: P2, JackStack: []card.Card{popJC, popJH}, JackOwners: []PlayerID{P1, P2}}},
			target:     Target{Owner: P2, Zone: ZonePoints, Index: 0},
			point:      seven,
			wantSide:   P1,
			wantStack:  []card.Card{popJC},
			wantOwners: []PlayerID{P1},
			popped:     popJH,
			wantP1:     15, wantP2: 0,
		},
		{
			// P1 pops its own Jack (legal: a 2 may target a Jack on either
			// side). The 9 returns to P2, taking P2 to 6 + 9 = 15 over its
			// King threshold of 14. P1 does not win and P2 does not win
			// off-turn; the turn passes and P2 wins at the start of its turn.
			name:       "ActorPopsOwnJack_OpponentOverThreshold_WinsAtStartOfTurn",
			actor:      P1,
			p1:         []PointEntry{{Card: nine, Owner: P2, JackStack: []card.Card{popJC}, JackOwners: []PlayerID{P1}}},
			p2:         []PointEntry{{Card: pc(card.Six, card.Diamonds), Owner: P2}},
			p2Perm:     []card.Card{popKC},
			target:     Target{Owner: P1, Zone: ZonePoints, Index: 0},
			point:      nine,
			wantSide:   P2,
			popped:     popJC,
			wantP1:     0,
			wantP2:     15,
			wantWinner: &p2,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := twoPlayerStart(nil, nil, nil)
			s.Active = c.actor
			s.Players[c.actor].Hand = []card.Card{popTwo}
			s.Players[P1].Points = c.p1
			s.Players[P2].Points = c.p2
			s.Players[P1].Permanents = c.p1Perm
			s.Players[P2].Permanents = c.p2Perm

			m, ok := findMove(LegalMoves(s), func(m Move) bool {
				return m.Kind == MoveOneOff && m.Card == popTwo && m.Target != nil && *m.Target == c.target
			})
			if !ok {
				t.Fatalf("2-as-scrap on %+v not offered by LegalMoves", c.target)
			}
			s1, err := Apply(s, m)
			if err != nil {
				t.Fatalf("apply 2: %v", err)
			}

			side, pe := findEntry(t, s1, c.point)
			if side != c.wantSide {
				t.Errorf("point %v sits on %v's side, want %v's", c.point, side, c.wantSide)
			}
			if pe.Controller() != c.wantSide {
				t.Errorf("Controller()=%v, want %v", pe.Controller(), c.wantSide)
			}
			if len(pe.JackStack) != len(c.wantStack) || len(pe.JackOwners) != len(c.wantOwners) {
				t.Fatalf("stack after pop = %+v / %+v, want %+v / %+v", pe.JackStack, pe.JackOwners, c.wantStack, c.wantOwners)
			}
			for i := range c.wantStack {
				if pe.JackStack[i] != c.wantStack[i] || pe.JackOwners[i] != c.wantOwners[i] {
					t.Errorf("stack after pop = %+v / %+v, want %+v / %+v", pe.JackStack, pe.JackOwners, c.wantStack, c.wantOwners)
					break
				}
			}

			// Scrap: exactly the 2 and the popped Jack; buried Jacks stay put.
			if len(s1.Scrap) != 2 || !scrapHas(s1, popTwo) || !scrapHas(s1, c.popped) {
				t.Errorf("scrap = %+v, want exactly [%v %v]", s1.Scrap, popTwo, c.popped)
			}
			for _, j := range c.wantStack {
				if scrapHas(s1, j) {
					t.Errorf("buried Jack %v must not be scrapped", j)
				}
			}

			if got := PointTotal(s1.Players[P1]); got != c.wantP1 {
				t.Errorf("P1 total = %d, want %d", got, c.wantP1)
			}
			if got := PointTotal(s1.Players[P2]); got != c.wantP2 {
				t.Errorf("P2 total = %d, want %d", got, c.wantP2)
			}

			if c.wantWinner != nil {
				if s1.Phase != PhaseGameOver || s1.Winner == nil || *s1.Winner != *c.wantWinner {
					t.Errorf("want Winner=%v in PhaseGameOver, got phase=%v winner=%v", *c.wantWinner, s1.Phase, s1.Winner)
				}
				// The winner is always the player whose turn it is: the actor
				// winning on its own action, or the opponent winning as its
				// turn begins.
				if s1.Active != *c.wantWinner {
					t.Errorf("winner %v should be the active player, Active=%v", *c.wantWinner, s1.Active)
				}
				if LegalMoves(s1) != nil {
					t.Errorf("game over must offer no moves, got %+v", LegalMoves(s1))
				}
			} else {
				if s1.Winner != nil || s1.Phase != PhaseNormal {
					t.Errorf("want no winner and PhaseNormal, got phase=%v winner=%v", s1.Phase, s1.Winner)
				}
				if s1.Active != c.actor.Other() {
					t.Errorf("turn should pass to %v, Active=%v", c.actor.Other(), s1.Active)
				}
			}
		})
	}
}

// Buried Jacks can't be targeted: a 3-Jack stack yields exactly one 2-move
// against it (the stack as a whole), and that move only ever pops the top.
func TestTwoPop_BuriedJacksNotTargetable(t *testing.T) {
	seven := pc(card.Seven, card.Diamonds)
	s := twoPlayerStart([]card.Card{popTwo}, nil, nil)
	s.Players[P2].Points = []PointEntry{
		{Card: seven, Owner: P2, JackStack: []card.Card{popJC, popJH, popJS}, JackOwners: []PlayerID{P1, P2, P1}},
	}
	// Invariant repair for setup: top Jack is P1's, so the entry belongs on P1.
	s.Players[P1].Points, s.Players[P2].Points = s.Players[P2].Points, nil

	var twoMoves []Move
	for _, m := range LegalMoves(s) {
		if m.Kind == MoveOneOff && m.Card == popTwo {
			twoMoves = append(twoMoves, m)
		}
	}
	if len(twoMoves) != 1 {
		t.Fatalf("want exactly one 2-move against the stack, got %d: %+v", len(twoMoves), twoMoves)
	}
	s1, err := Apply(s, twoMoves[0])
	if err != nil {
		t.Fatalf("apply 2: %v", err)
	}
	if !scrapHas(s1, popJS) || scrapHas(s1, popJC) || scrapHas(s1, popJH) {
		t.Errorf("only the top Jack (%v) may be scrapped, scrap=%+v", popJS, s1.Scrap)
	}
}

// The same ruling holds when the 2 resolves after the opponent declines a
// counter (resolvePending path rather than the immediate path). Legacy
// phase-1 state, built directly (see openLegacyWindow).
func TestTwoPop_ResolvesAfterCounterDeclined(t *testing.T) {
	seven := pc(card.Seven, card.Diamonds)
	oppTwo := pc(card.Two, card.Hearts)
	s := twoPlayerStart([]card.Card{popTwo}, []card.Card{oppTwo}, nil)
	s.Players[P2].Points = []PointEntry{
		{Card: seven, Owner: P2, JackStack: []card.Card{popJC, popJH}, JackOwners: []PlayerID{P1, P2}},
	}
	tgt := Target{Owner: P2, Zone: ZonePoints, Index: 0}
	s1 := openLegacyWindow(t, s, Move{Kind: MoveOneOff, Card: popTwo, HandIndex: 0, Target: &tgt})
	if s1.Phase != PhaseAwaitingCounter {
		t.Fatalf("expected PhaseAwaitingCounter, got %v", s1.Phase)
	}
	s2, err := Apply(s1, Move{Kind: MoveDecline})
	if err != nil {
		t.Fatalf("decline: %v", err)
	}
	side, pe := findEntry(t, s2, seven)
	if side != P1 || pe.Controller() != P1 {
		t.Errorf("after pop the 7 should sit on P1 under P1's Jack, side=%v controller=%v", side, pe.Controller())
	}
	if PointTotal(s2.Players[P1]) != 7 || PointTotal(s2.Players[P2]) != 0 {
		t.Errorf("totals P1=%d P2=%d, want 7/0", PointTotal(s2.Players[P1]), PointTotal(s2.Players[P2]))
	}
	if len(s2.Scrap) != 2 || !scrapHas(s2, popTwo) || !scrapHas(s2, popJH) || scrapHas(s2, popJC) {
		t.Errorf("scrap = %+v, want exactly [%v %v]", s2.Scrap, popTwo, popJH)
	}
	if s2.Phase != PhaseNormal || s2.Active != P2 {
		t.Errorf("want PhaseNormal with P2 next, got phase=%v active=%v", s2.Phase, s2.Active)
	}
}

// A countered 2 does nothing to the field: the Jack stack, the point card's
// side and both totals are untouched; only the two 2s reach the scrap.
func TestTwoPop_CounteredLeavesFieldUntouched(t *testing.T) {
	seven := pc(card.Seven, card.Diamonds)
	oppTwo := pc(card.Two, card.Hearts)
	s := twoPlayerStart([]card.Card{popTwo}, []card.Card{oppTwo}, nil)
	s.Players[P2].Points = []PointEntry{
		{Card: seven, Owner: P2, JackStack: []card.Card{popJC, popJH}, JackOwners: []PlayerID{P1, P2}},
	}
	// Invariant repair for setup: top Jack is P2's, so the entry stays on P2.
	tgt := Target{Owner: P2, Zone: ZonePoints, Index: 0}
	s1 := openLegacyWindow(t, s, Move{Kind: MoveOneOff, Card: popTwo, HandIndex: 0, Target: &tgt})
	s2, err := Apply(s1, Move{Kind: MoveCounter, Card: oppTwo, HandIndex: 0})
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	side, pe := findEntry(t, s2, seven)
	if side != P2 || pe.Controller() != P2 {
		t.Errorf("countered 2 must not move the 7, side=%v controller=%v", side, pe.Controller())
	}
	if len(pe.JackStack) != 2 || pe.JackStack[0] != popJC || pe.JackStack[1] != popJH ||
		len(pe.JackOwners) != 2 || pe.JackOwners[0] != P1 || pe.JackOwners[1] != P2 {
		t.Errorf("Jack stack changed: %+v / %+v", pe.JackStack, pe.JackOwners)
	}
	if PointTotal(s2.Players[P1]) != 0 || PointTotal(s2.Players[P2]) != 7 {
		t.Errorf("totals P1=%d P2=%d, want 0/7", PointTotal(s2.Players[P1]), PointTotal(s2.Players[P2]))
	}
	if len(s2.Scrap) != 2 || !scrapHas(s2, popTwo) || !scrapHas(s2, oppTwo) {
		t.Errorf("scrap = %+v, want exactly [%v %v]", s2.Scrap, popTwo, oppTwo)
	}
	if s2.Phase != PhaseNormal || s2.Active != P2 {
		t.Errorf("want PhaseNormal with P2 next, got phase=%v active=%v", s2.Phase, s2.Active)
	}
}

// Synthetic state pinning the branch where the controller after the pop is
// still the side holding the entry (two consecutive Jacks by the same player,
// which normal play does not produce): the card stays in place, at its index,
// and nothing is appended to either Points slice.
func TestTwoPop_ControllerUnchanged_CardDoesNotMove(t *testing.T) {
	seven := pc(card.Seven, card.Diamonds)
	three := pc(card.Three, card.Clubs)
	s := twoPlayerStart([]card.Card{popTwo}, nil, nil)
	s.Players[P2].Points = []PointEntry{
		{Card: three, Owner: P2},
		{Card: seven, Owner: P1, JackStack: []card.Card{popJC, popJH}, JackOwners: []PlayerID{P2, P2}},
	}
	tgt := Target{Owner: P2, Zone: ZonePoints, Index: 1}
	s1, err := Apply(s, Move{Kind: MoveOneOff, Card: popTwo, HandIndex: 0, Target: &tgt})
	if err != nil {
		t.Fatalf("apply 2: %v", err)
	}
	if len(s1.Players[P1].Points) != 0 || len(s1.Players[P2].Points) != 2 {
		t.Fatalf("points moved: P1=%+v P2=%+v", s1.Players[P1].Points, s1.Players[P2].Points)
	}
	pe := s1.Players[P2].Points[1]
	if pe.Card != seven || len(pe.JackStack) != 1 || pe.JackStack[0] != popJC ||
		len(pe.JackOwners) != 1 || pe.JackOwners[0] != P2 {
		t.Errorf("entry after pop = %+v, want the 7 at index 1 under [%v] owned by P2", pe, popJC)
	}
	if s1.Players[P2].Points[0].Card != three {
		t.Errorf("index 0 disturbed: %+v", s1.Players[P2].Points[0])
	}
	if !scrapHas(s1, popJH) || scrapHas(s1, popJC) || len(s1.Scrap) != 2 {
		t.Errorf("scrap = %+v, want exactly [%v %v]", s1.Scrap, popTwo, popJH)
	}
}

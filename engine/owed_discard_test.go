package engine

import (
	"reflect"
	"testing"

	"github.com/ApisMellow/cuttle/card"
)

// v0.3.0 owed discard (cuttle-web docs/counter-rewind-design.md §5): a 4
// adds 2 to Owed[opponent] and ends the turn. The opponent opens their turn
// in PhaseAwaitingDiscard (no Pending): they pay two cards at a time, or
// counter the 4's turn, then play on. Also the opening counter (Part B,
// ruled 2026-10-25) and the legacy paths (E9).

var (
	odFourS = pc(card.Four, card.Spades)  // Alice's first 4
	odFourH = pc(card.Four, card.Hearts)  // Alice's second 4
	odTwoD  = pc(card.Two, card.Diamonds) // Alice's counter-back 2
	odTenD  = pc(card.Ten, card.Diamonds)
	odSixD  = pc(card.Six, card.Diamonds)
	odTwoC  = pc(card.Two, card.Clubs) // Blake's 2s
	odTwoH  = pc(card.Two, card.Hearts)
	odSevD  = pc(card.Seven, card.Diamonds)
	odKingC = pc(card.King, card.Clubs)
	odAceC  = pc(card.Ace, card.Clubs)
	odNineS = pc(card.Nine, card.Spades)
)

// owedDeal is the dealt position: Alice (P1) to open.
func owedDeal(p1Hand, p2Hand []card.Card) GameState {
	return twoPlayerStart(p1Hand, p2Hand, []card.Card{
		pc(card.Queen, card.Hearts), pc(card.Three, card.Hearts), pc(card.Five, card.Clubs),
		pc(card.King, card.Hearts), pc(card.Nine, card.Clubs), pc(card.Eight, card.Hearts),
	})
}

func owedFullDeal() GameState {
	return owedDeal([]card.Card{odFourS, odFourH, odTwoD, odTenD, odSixD},
		[]card.Card{odTwoC, odTwoH, odSevD, odKingC, odAceC})
}

func playCard(t *testing.T, s GameState, kind MoveKind, c card.Card) GameState {
	t.Helper()
	return mustApply(t, s, Move{Kind: kind, Card: c, HandIndex: handIndexOf(s.Players[s.Active].Hand, c)})
}

func payPair(t *testing.T, s GameState, a, b card.Card) GameState {
	t.Helper()
	h := s.Players[s.Active].Hand
	return mustApply(t, s, Move{Kind: MoveDiscardPair, DiscardA: handIndexOf(h, a), DiscardB: handIndexOf(h, b)})
}

func wantOwedStep(t *testing.T, s GameState, who PlayerID, owed [2]int) {
	t.Helper()
	if s.Phase != PhaseAwaitingDiscard || s.Active != who || s.Pending != nil || s.Owed != owed {
		t.Fatalf("want owed-discard step for P%d owed=%v, got phase=%v active=%v pending=%v owed=%v",
			who+1, owed, s.Phase, s.Active, s.Pending, s.Owed)
	}
}

// E7 basic: nothing is discarded in Alice's turn; Blake's turn opens on the
// debt with the pairs in order and his counters appended; paying keeps the
// turn with Blake and removes the counter option.
func TestOwed_E7_Basic(t *testing.T) {
	s1 := playCard(t, owedFullDeal(), MoveOneOff, odFourS)
	wantOwedStep(t, s1, P2, [2]int{0, 2})
	if len(s1.Players[P2].Hand) != 5 || !reflect.DeepEqual(s1.Scrap, []card.Card{odFourS}) {
		t.Fatalf("nothing may be discarded in Alice's turn: hand=%v scrap=%v", s1.Players[P2].Hand, s1.Scrap)
	}
	if s1.TurnEnd[P1] == nil || s1.TurnEnd[P1].Owed != [2]int{0, 2} {
		t.Fatalf("the snapshot at the end of the 4's turn must hold the debt")
	}
	moves := LegalMoves(s1)
	pairs := legalDiscardMoves(s1)
	if len(pairs) != 10 || len(moves) != 12 || !reflect.DeepEqual(moves[:10], pairs) {
		t.Fatalf("want the 10 pairs in today's order, then 2 counters; got %+v", moves)
	}
	if moves[10] != (Move{Kind: MoveCounter, Card: odTwoC, HandIndex: 0}) ||
		moves[11] != (Move{Kind: MoveCounter, Card: odTwoH, HandIndex: 1}) {
		t.Errorf("counters = %+v, want 2♣ then 2♥", moves[10:])
	}
	for _, bad := range []Move{{Kind: MoveDraw}, {Kind: MovePlayPoint, Card: odSevD, HandIndex: 2}} {
		if _, err := Apply(s1, bad); err == nil {
			t.Errorf("%+v must be illegal before paying", bad)
		}
	}

	paid := payPair(t, s1, odSevD, odKingC)
	if paid.Phase != PhaseNormal || paid.Active != P2 || paid.Owed != [2]int{} {
		t.Fatalf("after paying: phase=%v active=%v owed=%v, want PhaseNormal, P2, no debt", paid.Phase, paid.Active, paid.Owed)
	}
	if !reflect.DeepEqual(paid.Players[P2].Hand, []card.Card{odTwoC, odTwoH, odAceC}) ||
		!reflect.DeepEqual(paid.Scrap, []card.Card{odFourS, odSevD, odKingC}) {
		t.Errorf("hand=%v scrap=%v", paid.Players[P2].Hand, paid.Scrap)
	}
	if paid.TurnEnd != s1.TurnEnd || paid.LastTurn.Counterable {
		t.Errorf("the turn must not end, and paying lets Alice's turn stand: last=%+v", paid.LastTurn)
	}
	if cm := counterMoves(paid); len(cm) != 0 {
		t.Errorf("no counter after paying, got %+v", cm)
	}
	if _, err := Apply(paid, Move{Kind: MoveCounter, Card: odTwoC, HandIndex: 0}); err == nil {
		t.Errorf("Apply must reject a counter after paying")
	}
	// Blake plays on in the same turn.
	next := playCard(t, paid, MoveOneOff, odAceC)
	if next.Active != P1 || next.Phase != PhaseNormal {
		t.Errorf("Blake's one-off should end his turn, active=%v phase=%v", next.Active, next.Phase)
	}
}

// E7 counter: the restore brings back Owed[B] = 0; the debt disappears.
func TestOwed_E7_CounterErasesDebt(t *testing.T) {
	s1 := playCard(t, owedFullDeal(), MoveOneOff, odFourS)
	c1 := mustCounter(t, s1, odTwoC)
	wantRestore(t, s1, c1, P2, []card.Card{odFourS}, odTwoC)
	if c1.Owed != [2]int{} || c1.Phase != PhaseNormal || c1.Active != P2 {
		t.Errorf("owed=%v phase=%v active=%v, want no debt, PhaseNormal, P2", c1.Owed, c1.Phase, c1.Active)
	}
}

// E7 counter-back: Alice's counter-back restores the end of her 4 turn, so
// the debt returns and waits for Blake's next turn start, where he can
// counter again (back to no debt) or pay.
func TestOwed_E7_CounterBackDebtReturns(t *testing.T) {
	s1 := playCard(t, owedFullDeal(), MoveOneOff, odFourS)
	c1 := mustCounter(t, s1, odTwoC)
	t1 := playCard(t, c1, MovePlayPoint, odSevD) // Blake's normal action
	c2 := mustCounter(t, t1, odTwoD)
	wantRestore(t, t1, c2, P1, []card.Card{odTwoC}, odTwoD)
	if c2.Owed != [2]int{0, 2} || c2.Phase != PhaseNormal || c2.Active != P1 {
		t.Fatalf("counter-back: owed=%v phase=%v active=%v, want Blake owing 2 and Alice playing", c2.Owed, c2.Phase, c2.Active)
	}
	t2 := playCard(t, c2, MovePlayPoint, odTenD)
	wantOwedStep(t, t2, P2, [2]int{0, 2})
	if !reflect.DeepEqual(t2.Players[P2].Hand, []card.Card{odTwoH, odSevD, odKingC, odAceC}) {
		t.Fatalf("Blake's hand = %v", t2.Players[P2].Hand)
	}
	if cm := counterMoves(t2); len(cm) != 1 || cm[0].Card != odTwoH {
		t.Fatalf("Blake may counter the counter-back turn with 2♥, got %+v", cm)
	}
	c3 := mustCounter(t, t2, odTwoH)
	wantRestore(t, t2, c3, P2, []card.Card{odTwoD}, odTwoH)
	if c3.Owed != [2]int{} {
		t.Errorf("countering the counter-back clears the debt, owed=%v", c3.Owed)
	}
	// Or Blake pays instead.
	p3 := payPair(t, t2, odKingC, odAceC)
	if p3.Phase != PhaseNormal || p3.Active != P2 || p3.Owed != [2]int{} {
		t.Errorf("paying: phase=%v active=%v owed=%v", p3.Phase, p3.Active, p3.Owed)
	}
}

// E7 stacking: only a counter-back followed by a second 4 stacks. Owed
// becomes 4, paid two at a time; a counter there clears both.
func TestOwed_E7_Stacking(t *testing.T) {
	s1 := playCard(t, owedFullDeal(), MoveOneOff, odFourS)
	c1 := mustCounter(t, s1, odTwoC)
	t1 := playCard(t, c1, MovePlayPoint, odSevD)
	c2 := mustCounter(t, t1, odTwoD)
	t2 := playCard(t, c2, MoveOneOff, odFourH)
	wantOwedStep(t, t2, P2, [2]int{0, 4})

	t.Run("PayTwoAtATime", func(t *testing.T) {
		p1 := payPair(t, t2, odSevD, odKingC)
		wantOwedStep(t, p1, P2, [2]int{0, 2})
		if cm := counterMoves(p1); len(cm) != 0 {
			t.Errorf("no counter after the first pair, got %+v", cm)
		}
		p2 := payPair(t, p1, odTwoH, odAceC)
		if p2.Phase != PhaseNormal || p2.Active != P2 || p2.Owed != [2]int{} || len(p2.Players[P2].Hand) != 0 {
			t.Errorf("after both pairs: phase=%v active=%v owed=%v hand=%v", p2.Phase, p2.Active, p2.Owed, p2.Players[P2].Hand)
		}
		if p2.TurnEnd != t2.TurnEnd {
			t.Errorf("paying must not end the turn")
		}
	})
	t.Run("CounterClearsBoth", func(t *testing.T) {
		c3 := mustCounter(t, t2, odTwoH)
		wantRestore(t, t2, c3, P2, []card.Card{odTwoD, odFourH}, odTwoH)
		if c3.Owed != [2]int{} {
			t.Errorf("owed=%v, want both 4s erased", c3.Owed)
		}
	})
	t.Run("PlainSequenceNeverStacks", func(t *testing.T) {
		p := payPair(t, s1, odSevD, odKingC)
		p = playCard(t, p, MovePlayPoint, odAceC)
		p = playCard(t, p, MoveOneOff, odFourH)
		wantOwedStep(t, p, P2, [2]int{0, 2})
	})
}

// E7 paying again: Blake pays with 7♦ and K♣, then plays a one-off; Alice
// counters that turn, so the debt and 7♦ K♣ come back to Blake.
func TestOwed_E7_PayAgainAfterCounteredPayingTurn(t *testing.T) {
	s1 := playCard(t, owedFullDeal(), MoveOneOff, odFourS)
	paid := payPair(t, s1, odSevD, odKingC)
	t1 := playCard(t, paid, MoveOneOff, odAceC)
	c2 := mustCounter(t, t1, odTwoD)
	wantRestore(t, t1, c2, P1, []card.Card{odAceC}, odTwoD)
	if c2.Owed != [2]int{0, 2} {
		t.Fatalf("owed=%v, want Blake owing 2 again", c2.Owed)
	}
	if !reflect.DeepEqual(c2.Players[P2].Hand, []card.Card{odTwoC, odTwoH, odSevD, odKingC}) {
		t.Errorf("7♦ and K♣ should be back in Blake's hand, hand=%v", c2.Players[P2].Hand)
	}
	t2 := playCard(t, c2, MovePlayPoint, odTenD)
	wantOwedStep(t, t2, P2, [2]int{0, 2})
	// He may pay with different cards this time.
	p := payPair(t, t2, odTwoC, odTwoH)
	if !reflect.DeepEqual(p.Players[P2].Hand, []card.Card{odSevD, odKingC}) || p.Phase != PhaseNormal {
		t.Errorf("hand=%v phase=%v", p.Players[P2].Hand, p.Phase)
	}
}

// E7 small hands, plus the E1 twins for a 4.
func TestOwed_E7_SmallHands(t *testing.T) {
	t.Run("ZeroCards", func(t *testing.T) {
		s := playCard(t, owedDeal([]card.Card{odFourS}, nil), MoveOneOff, odFourS)
		if s.Phase != PhaseNormal || s.Active != P2 || s.Owed != [2]int{} {
			t.Errorf("empty hand pays nothing: phase=%v active=%v owed=%v", s.Phase, s.Active, s.Owed)
		}
		if s.TurnEnd[P1].Owed != [2]int{0, 2} {
			t.Errorf("the 4's snapshot still records the debt, got %v", s.TurnEnd[P1].Owed)
		}
	})
	t.Run("OneCard", func(t *testing.T) {
		s := playCard(t, owedDeal([]card.Card{odFourS}, []card.Card{odNineS}), MoveOneOff, odFourS)
		wantOwedStep(t, s, P2, [2]int{0, 2})
		moves := LegalMoves(s)
		if len(moves) != 1 || moves[0] != (Move{Kind: MoveDiscardPair, DiscardA: 0, DiscardB: -1}) {
			t.Fatalf("want the single discard, got %+v", moves)
		}
		p := mustApply(t, s, moves[0])
		if p.Phase != PhaseNormal || p.Active != P2 || p.Owed != [2]int{} || len(p.Players[P2].Hand) != 0 {
			t.Errorf("phase=%v active=%v owed=%v hand=%v", p.Phase, p.Active, p.Owed, p.Players[P2].Hand)
		}
		if lm := LegalMoves(p); len(lm) != 1 || lm[0].Kind != MoveDraw {
			t.Errorf("Blake's turn continues with a draw, got %+v", lm)
		}
	})
	t.Run("TwoCards", func(t *testing.T) {
		s := playCard(t, owedDeal([]card.Card{odFourS}, []card.Card{odNineS, odSevD}), MoveOneOff, odFourS)
		moves := LegalMoves(s)
		if len(moves) != 1 || moves[0] != (Move{Kind: MoveDiscardPair, DiscardA: 0, DiscardB: 1}) {
			t.Fatalf("want the one pair, got %+v", moves)
		}
		p := mustApply(t, s, moves[0])
		if p.Phase != PhaseNormal || p.Active != P2 || len(p.Players[P2].Hand) != 0 {
			t.Errorf("phase=%v active=%v hand=%v", p.Phase, p.Active, p.Players[P2].Hand)
		}
	})
	t.Run("DebtBiggerThanHand", func(t *testing.T) {
		deal := owedDeal([]card.Card{odFourS}, []card.Card{odNineS, odSevD, odKingC})
		deal.Owed = [2]int{0, 2} // a debt already standing, as after a counter-back
		s := playCard(t, deal, MoveOneOff, odFourS)
		wantOwedStep(t, s, P2, [2]int{0, 4})
		s = payPair(t, s, odNineS, odSevD)
		wantOwedStep(t, s, P2, [2]int{0, 2})
		s = mustApply(t, s, Move{Kind: MoveDiscardPair, DiscardA: 0, DiscardB: -1})
		if s.Phase != PhaseNormal || s.Active != P2 || s.Owed != [2]int{} || len(s.Players[P2].Hand) != 0 {
			t.Errorf("whole hand discarded, debt cleared: phase=%v active=%v owed=%v hand=%v", s.Phase, s.Active, s.Owed, s.Players[P2].Hand)
		}
	})
	t.Run("CounterAndKeepTheSeven", func(t *testing.T) {
		s := playCard(t, owedDeal([]card.Card{odFourS}, []card.Card{odTwoC, odSevD}), MoveOneOff, odFourS)
		moves := LegalMoves(s)
		want := []Move{{Kind: MoveDiscardPair, DiscardA: 0, DiscardB: 1}, {Kind: MoveCounter, Card: odTwoC, HandIndex: 0}}
		if !reflect.DeepEqual(moves, want) {
			t.Fatalf("moves = %+v, want %+v", moves, want)
		}
		c := mustApply(t, s, moves[1])
		if !reflect.DeepEqual(c.Players[P2].Hand, []card.Card{odSevD}) || c.Owed != [2]int{} || c.Phase != PhaseNormal || c.Active != P2 {
			t.Errorf("hand=%v owed=%v phase=%v active=%v", c.Players[P2].Hand, c.Owed, c.Phase, c.Active)
		}
	})
	t.Run("E1_Twins", func(t *testing.T) {
		a := playCard(t, owedDeal([]card.Card{odFourS}, []card.Card{odTwoC, odSevD}), MoveOneOff, odFourS)
		b := playCard(t, owedDeal([]card.Card{odFourS}, []card.Card{pc(card.Seven, card.Clubs), odSevD}), MoveOneOff, odFourS)
		if a.Phase != b.Phase || a.Active != b.Active || a.Owed != b.Owed {
			t.Errorf("with 2 phase=%v active=%v owed=%v, without phase=%v active=%v owed=%v",
				a.Phase, a.Active, a.Owed, b.Phase, b.Active, b.Owed)
		}
	})
}

// Part B: the second player may counter the opening one-off; the rewind
// point is the dealt position.
func TestOpeningCounter(t *testing.T) {
	ace := pc(card.Ace, card.Spades)
	deal := owedDeal([]card.Card{ace, odTenD}, []card.Card{odTwoC, odSevD})
	deal.Players[P1].Points = []PointEntry{{Card: pc(card.Seven, card.Hearts), Owner: P1}}

	t.Run("SnapshotIsDealtPosition", func(t *testing.T) {
		s1 := playCard(t, deal, MoveOneOff, ace)
		if s1.TurnEnd[P2] == nil || !reflect.DeepEqual(clone(*s1.TurnEnd[P2]), clone(deal)) {
			t.Fatalf("TurnEnd[P2] should be the dealt position, got %+v", s1.TurnEnd[P2])
		}
		if deal.TurnEnd != [2]*GameState{} {
			t.Errorf("Apply must not touch its input's TurnEnd")
		}
	})
	t.Run("CounterRestoresDealPlusScrap", func(t *testing.T) {
		s1 := playCard(t, deal, MoveOneOff, ace)
		got := mustCounter(t, s1, odTwoC)
		want := clone(deal)
		want.Players[P1].Hand = []card.Card{odTenD}
		want.Players[P2].Hand = []card.Card{odSevD}
		want.Scrap = []card.Card{ace, odTwoC}
		want.Active = P2
		want.ThisTurn = TurnRecord{Counterable: true, Countered: []card.Card{odTwoC}}
		want.TurnEnd = s1.TurnEnd
		if !reflect.DeepEqual(clone(want), clone(got)) {
			t.Errorf("restore mismatch\n want %+v\n  got %+v", want, got)
		}
	})
	t.Run("NonCounterableOpeningOffersNothing", func(t *testing.T) {
		s1 := playCard(t, deal, MovePlayPoint, odTenD)
		if s1.TurnEnd[P2] == nil {
			t.Fatalf("the dealt position is saved on any first move")
		}
		if cm := counterMoves(s1); len(cm) != 0 {
			t.Errorf("no counter after a point play, got %+v", cm)
		}
	})
}

// E9: the v0.2.x phase-3 shape (Pending set, discard inside the 4-player's
// turn) still completes. The legacy phase-1 paths are covered by
// counter_test.go: TestCounter_Decline_ResolvesImmediately (Decline) and
// TestCounter_AceVsTwoThenTwo_Resolves / TestCounter_AceVsOneTwo_Cancelled
// (counter chains).
func TestLegacy_E9_OldStyleDiscardCompletes(t *testing.T) {
	s := owedDeal(nil, []card.Card{odTwoC, odSevD, odNineS})
	s.Scrap = []card.Card{odFourS}
	s.Pending = &PendingOneOff{PlayedBy: P1, Card: odFourS}
	s.Phase = PhaseAwaitingDiscard
	s.Active = P2
	// Legacy saves carry no turn records; even with one, no counter is
	// offered in the old shape.
	s.LastTurn = TurnRecord{Counterable: true}
	moves := LegalMoves(s)
	if !reflect.DeepEqual(moves, legalDiscardMoves(s)) || len(moves) != 3 {
		t.Fatalf("legacy phase 3 lists only the pairs, got %+v", moves)
	}
	if _, err := Apply(s, Move{Kind: MoveCounter, Card: odTwoC, HandIndex: 0}); err == nil {
		t.Errorf("no counter in the legacy discard")
	}
	out := mustApply(t, s, Move{Kind: MoveDiscardPair, DiscardA: 1, DiscardB: 2})
	if out.Phase != PhaseNormal || out.Active != P2 || out.Pending != nil || out.Owed != [2]int{} {
		t.Errorf("phase=%v active=%v pending=%v owed=%v, want Blake's normal turn", out.Phase, out.Active, out.Pending, out.Owed)
	}
	if !reflect.DeepEqual(out.Players[P2].Hand, []card.Card{odTwoC}) ||
		!reflect.DeepEqual(out.Scrap, []card.Card{odFourS, odSevD, odNineS}) {
		t.Errorf("hand=%v scrap=%v", out.Players[P2].Hand, out.Scrap)
	}
}

// E9: a legacy window on a 4 that resolves (Decline) finishes under the
// owed-discard rule: the 4-player's turn ends and the debt opens Blake's turn.
func TestLegacy_E9_FourWindowDeclineOwes(t *testing.T) {
	s := owedDeal([]card.Card{odFourS}, []card.Card{odTwoC, odSevD, odNineS})
	s1 := openLegacyWindow(t, s, Move{Kind: MoveOneOff, Card: odFourS, HandIndex: 0})
	out := mustApply(t, s1, Move{Kind: MoveDecline})
	wantOwedStep(t, out, P2, [2]int{0, 2})
	if cm := counterMoves(out); len(cm) != 0 {
		t.Errorf("Blake declined: no counter at his turn start, got %+v", cm)
	}
}

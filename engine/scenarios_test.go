package engine

import (
	"testing"

	"github.com/ApisMellow/cuttle/card"
)

// Task 23: Scenario fixtures. End-to-end witnesses that tie together
// several rules at once. Per-card tests remain the source of truth for
// individual effects; these scenarios just verify that composed states
// behave as the design conversation described.

func assertDeckCount(t *testing.T, s GameState, want int) {
	t.Helper()
	count := len(s.Deck) + len(s.Scrap)
	for i := 0; i < 2; i++ {
		count += len(s.Players[i].Hand)
		count += len(s.Players[i].Permanents)
		for _, pe := range s.Players[i].Points {
			count += 1 + len(pe.JackStack)
		}
	}
	if s.Pending != nil {
		count += len(s.Pending.Revealed)
		count += len(s.Pending.CounterChain)
		// the Pending.Card is the played one-off, not yet in scrap
		count++
	}
	if count != want {
		t.Errorf("card accounting: got %d, want %d", count, want)
	}
}

// cardsInUse collects every card visible anywhere in s EXCEPT the Deck,
// so padDeckTo52 can fill the deck with the remaining cards of a full 52.
func cardsInUse(s GameState) map[card.Card]bool {
	used := map[card.Card]bool{}
	for _, c := range s.Scrap {
		used[c] = true
	}
	for i := 0; i < 2; i++ {
		for _, c := range s.Players[i].Hand {
			used[c] = true
		}
		for _, c := range s.Players[i].Permanents {
			used[c] = true
		}
		for _, pe := range s.Players[i].Points {
			used[pe.Card] = true
			for _, j := range pe.JackStack {
				used[j] = true
			}
		}
	}
	if s.Pending != nil {
		used[s.Pending.Card] = true
		for _, c := range s.Pending.Revealed {
			used[c] = true
		}
		for _, c := range s.Pending.CounterChain {
			used[c] = true
		}
	}
	for _, c := range s.Deck {
		used[c] = true
	}
	return used
}

// padDeckTo52 appends every card from a fresh 52-card deck that is not
// already present somewhere in s onto s.Deck, so that assertDeckCount
// sees a total of 52 regardless of how many cards the scenario pins in
// specific zones.
func padDeckTo52(s *GameState) {
	used := cardsInUse(*s)
	for r := card.Ace; r <= card.King; r++ {
		for _, suit := range []card.Suit{card.Clubs, card.Diamonds, card.Hearts, card.Spades} {
			c := card.Card{Rank: r, Suit: suit}
			if used[c] {
				continue
			}
			s.Deck = append(s.Deck, c)
		}
	}
}

func findMove(moves []Move, pred func(Move) bool) (Move, bool) {
	for _, m := range moves {
		if pred(m) {
			return m, true
		}
	}
	return Move{}, false
}

// Scenario 1: the adversary scenario from the design conversation.
// P1 plays 9♥ as a point card, reaching 21 (no Kings → threshold 21).
func TestScenario_Adversary9HWin(t *testing.T) {
	nineH := card.Card{Rank: card.Nine, Suit: card.Hearts}
	s := GameState{
		Players: [2]PlayerState{
			{
				Hand: []card.Card{
					{Rank: card.King, Suit: card.Spades},
					{Rank: card.Three, Suit: card.Clubs},
					nineH,
					{Rank: card.Two, Suit: card.Diamonds},
				},
				Points: []PointEntry{
					{Card: card.Card{Rank: card.Seven, Suit: card.Hearts}, Owner: P1},
					{Card: card.Card{Rank: card.Five, Suit: card.Clubs}, Owner: P1},
				},
				Permanents: []card.Card{{Rank: card.Queen, Suit: card.Diamonds}},
			},
			{
				Hand: []card.Card{
					{Rank: card.Jack, Suit: card.Hearts},
					{Rank: card.Six, Suit: card.Spades},
					{Rank: card.Ace, Suit: card.Clubs},
					{Rank: card.Two, Suit: card.Spades},
					{Rank: card.Five, Suit: card.Diamonds},
				},
				Points: []PointEntry{
					{Card: card.Card{Rank: card.Ten, Suit: card.Diamonds}, Owner: P2},
					{Card: card.Card{Rank: card.Four, Suit: card.Spades}, Owner: P2},
				},
				Permanents: []card.Card{{Rank: card.Eight, Suit: card.Clubs}},
			},
		},
		Active: P1,
		Phase:  PhaseNormal,
	}
	padDeckTo52(&s)
	assertDeckCount(t, s, 52)

	moves := LegalMoves(s)
	play9, ok := findMove(moves, func(m Move) bool {
		return m.Kind == MovePlayPoint && m.Card == nineH
	})
	if !ok {
		t.Fatal("expected LegalMoves to include 'play 9H as point'")
	}

	s2, err := Apply(s, play9)
	if err != nil {
		t.Fatalf("apply 9H point: %v", err)
	}
	if s2.Phase != PhaseGameOver {
		t.Errorf("expected PhaseGameOver, got %v", s2.Phase)
	}
	if s2.Winner == nil || *s2.Winner != P1 {
		t.Errorf("expected P1 winner, got %v", s2.Winner)
	}
	if PointTotal(s2.Players[P1]) < 21 {
		t.Errorf("expected P1 >= 21, got %d", PointTotal(s2.Players[P1]))
	}
	assertDeckCount(t, s2, 52)
}

// Scenario 2: counter chain of length 3 (2 vs 2 vs 2) cancels the Ace.
// All four one-off cards (Ace + three 2s) land in scrap; no other board
// state changes.
func TestScenario_CounterChainTriple(t *testing.T) {
	ace := card.Card{Rank: card.Ace, Suit: card.Spades}
	twoP1 := card.Card{Rank: card.Two, Suit: card.Clubs}
	twoP2a := card.Card{Rank: card.Two, Suit: card.Hearts}
	twoP2b := card.Card{Rank: card.Two, Suit: card.Diamonds}

	s := GameState{
		Players: [2]PlayerState{
			{
				Hand: []card.Card{ace, twoP1},
				Points: []PointEntry{
					{Card: card.Card{Rank: card.Seven, Suit: card.Hearts}, Owner: P1},
				},
			},
			{
				Hand: []card.Card{twoP2a, twoP2b},
				Points: []PointEntry{
					{Card: card.Card{Rank: card.Five, Suit: card.Diamonds}, Owner: P2},
				},
			},
		},
		Active: P1,
		Phase:  PhaseNormal,
	}
	padDeckTo52(&s)
	assertDeckCount(t, s, 52)

	// Invariant: before playing, "play Ace as one-off" must be legal.
	moves := LegalMoves(s)
	playAce, ok := findMove(moves, func(m Move) bool {
		return m.Kind == MoveOneOff && m.Card == ace
	})
	if !ok {
		t.Fatal("expected MoveOneOff Ace in legal moves")
	}

	// P1 plays Ace → P2 awaiting counter. Legacy phase-1 path (v0.2.x
	// mid-window save), built directly; new play resolves the Ace at once.
	s1 := openLegacyWindow(t, s, playAce)
	if s1.Phase != PhaseAwaitingCounter || s1.Active != P2 {
		t.Fatalf("after Ace: phase=%v active=%v", s1.Phase, s1.Active)
	}
	assertDeckCount(t, s1, 52)

	// P2 counters with one 2.
	s2, err := Apply(s1, Move{Kind: MoveCounter, Card: twoP2a, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	if s2.Phase != PhaseAwaitingCounter || s2.Active != P1 {
		t.Fatalf("after P2 counter: phase=%v active=%v", s2.Phase, s2.Active)
	}

	// P1 counter-counters.
	s3, err := Apply(s2, Move{Kind: MoveCounter, Card: twoP1, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	if s3.Phase != PhaseAwaitingCounter || s3.Active != P2 {
		t.Fatalf("after P1 counter: phase=%v active=%v", s3.Phase, s3.Active)
	}

	// P2 counter-counter-counters with its remaining 2. P1 has no 2s left,
	// so this auto-resolves. Chain length 3 → odd → Ace cancelled.
	s4, err := Apply(s3, Move{Kind: MoveCounter, Card: twoP2b, HandIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	if s4.Phase != PhaseNormal {
		t.Errorf("expected PhaseNormal, got %v", s4.Phase)
	}
	// Points untouched on both sides.
	if len(s4.Players[P1].Points) != 1 || s4.Players[P1].Points[0].Card.Rank != card.Seven {
		t.Errorf("P1 points should be unchanged, got %+v", s4.Players[P1].Points)
	}
	if len(s4.Players[P2].Points) != 1 || s4.Players[P2].Points[0].Card.Rank != card.Five {
		t.Errorf("P2 points should be unchanged, got %+v", s4.Players[P2].Points)
	}
	// Hands drained of the four counter-chain cards.
	if len(s4.Players[P1].Hand) != 0 || len(s4.Players[P2].Hand) != 0 {
		t.Errorf("hands should be empty, got P1=%v P2=%v", s4.Players[P1].Hand, s4.Players[P2].Hand)
	}
	// Scrap: Ace + three 2s.
	if len(s4.Scrap) != 4 {
		t.Errorf("expected 4 scrap cards, got %d", len(s4.Scrap))
	}
	assertDeckCount(t, s4, 52)
}

// Scenario 3: Jack chain-steal. P1 already controls a 7♥ originally owned
// by P2 (via a Jack). P2 plays a second Jack onto the same point. The
// PointEntry transplants to P2 with both Jacks in the stack, and P2's
// point total reflects the 7.
func TestScenario_JackChainSteal(t *testing.T) {
	firstJack := card.Card{Rank: card.Jack, Suit: card.Hearts} // P1's original steal
	secondJack := card.Card{Rank: card.Jack, Suit: card.Spades}
	seven := card.Card{Rank: card.Seven, Suit: card.Hearts}

	s := GameState{
		Players: [2]PlayerState{
			{
				// P1 controls the stolen 7♥ with Owner=P2.
				Points: []PointEntry{
					{
						Card:       seven,
						Owner:      P2,
						JackStack:  []card.Card{firstJack},
						JackOwners: []PlayerID{P1},
					},
				},
			},
			{
				Hand: []card.Card{secondJack},
			},
		},
		Active: P2,
		Phase:  PhaseNormal,
	}
	padDeckTo52(&s)
	assertDeckCount(t, s, 52)

	moves := LegalMoves(s)
	playJack, ok := findMove(moves, func(m Move) bool {
		return m.Kind == MovePlayPermanent && m.Card == secondJack &&
			m.JackTarget != nil && m.JackTarget.Owner == P1 &&
			m.JackTarget.Zone == ZonePoints && m.JackTarget.Index == 0
	})
	if !ok {
		t.Fatal("expected MovePlayPermanent Jack targeting P1's stolen point")
	}

	s2, err := Apply(s, playJack)
	if err != nil {
		t.Fatal(err)
	}
	// The entry transplants to P2.
	if len(s2.Players[P1].Points) != 0 {
		t.Errorf("P1 should no longer control the point, got %d", len(s2.Players[P1].Points))
	}
	if len(s2.Players[P2].Points) != 1 {
		t.Fatalf("P2 should control one point, got %d", len(s2.Players[P2].Points))
	}
	pe := s2.Players[P2].Points[0]
	if pe.Card != seven {
		t.Errorf("point card should be 7♥, got %v", pe.Card)
	}
	if len(pe.JackStack) != 2 || pe.JackStack[0] != firstJack || pe.JackStack[1] != secondJack {
		t.Errorf("JackStack should be [J♥, J♠], got %v", pe.JackStack)
	}
	if len(pe.JackOwners) != 2 || pe.JackOwners[0] != P1 || pe.JackOwners[1] != P2 {
		t.Errorf("JackOwners should be [P1, P2], got %v", pe.JackOwners)
	}
	if pe.Owner != P2 {
		t.Errorf("original owner should still be P2, got %v", pe.Owner)
	}
	if PointTotal(s2.Players[P2]) != 7 {
		t.Errorf("P2 point total should be 7, got %d", PointTotal(s2.Players[P2]))
	}
	assertDeckCount(t, s2, 52)
}

// Scenario 4: Six wipes mixed permanents. P1 has Q+K+glasses-8; P2 has
// Q + a Jack stealing a P1 point. After P1 plays a Six, both sides'
// permanents are scrapped, and the stolen point returns to P1.
func TestScenario_SixWipesMixedPermanents(t *testing.T) {
	six := card.Card{Rank: card.Six, Suit: card.Clubs}
	stolenPoint := card.Card{Rank: card.Nine, Suit: card.Clubs} // originally P1's
	stealJack := card.Card{Rank: card.Jack, Suit: card.Diamonds}

	s := GameState{
		Players: [2]PlayerState{
			{
				Hand: []card.Card{six},
				Permanents: []card.Card{
					{Rank: card.Queen, Suit: card.Spades},
					{Rank: card.King, Suit: card.Hearts},
					{Rank: card.Eight, Suit: card.Clubs},
				},
			},
			{
				// P2 controls a stolen P1 point via a Jack, plus a Queen perm.
				Permanents: []card.Card{
					{Rank: card.Queen, Suit: card.Hearts},
				},
				Points: []PointEntry{
					{
						Card:       stolenPoint,
						Owner:      P1,
						JackStack:  []card.Card{stealJack},
						JackOwners: []PlayerID{P2},
					},
				},
			},
		},
		Active: P1,
		Phase:  PhaseNormal,
	}
	padDeckTo52(&s)
	assertDeckCount(t, s, 52)

	moves := LegalMoves(s)
	playSix, ok := findMove(moves, func(m Move) bool {
		return m.Kind == MoveOneOff && m.Card == six
	})
	if !ok {
		t.Fatal("expected MoveOneOff Six in legal moves")
	}

	s2, err := Apply(s, playSix)
	if err != nil {
		t.Fatal(err)
	}
	// P2 has no 2s, so this should resolve immediately.
	if s2.Phase != PhaseNormal {
		t.Errorf("expected PhaseNormal, got %v", s2.Phase)
	}
	if len(s2.Players[P1].Permanents) != 0 {
		t.Errorf("P1 permanents should be empty, got %v", s2.Players[P1].Permanents)
	}
	if len(s2.Players[P2].Permanents) != 0 {
		t.Errorf("P2 permanents should be empty, got %v", s2.Players[P2].Permanents)
	}
	// Stolen point returns to P1.
	if len(s2.Players[P2].Points) != 0 {
		t.Errorf("P2 should no longer control stolen point, got %d", len(s2.Players[P2].Points))
	}
	if len(s2.Players[P1].Points) != 1 {
		t.Fatalf("P1 should have returned point, got %d", len(s2.Players[P1].Points))
	}
	back := s2.Players[P1].Points[0]
	if back.Card != stolenPoint || back.Owner != P1 {
		t.Errorf("returned point wrong: %+v", back)
	}
	if len(back.JackStack) != 0 {
		t.Errorf("returned point should have no jacks, got %v", back.JackStack)
	}
	// Scrap contents: six, Q♠, K♥, 8♣, Q♥, J♦ = 6 cards.
	if len(s2.Scrap) != 6 {
		t.Errorf("expected 6 scrap cards, got %d", len(s2.Scrap))
	}
	assertDeckCount(t, s2, 52)
}

// Scenario 5: Ace wipes ALL points on both sides even when a Queen is in
// play. The Queen only protects targeted effects; board wipes still work.
func TestScenario_AceWipeWithQueenPresent(t *testing.T) {
	ace := card.Card{Rank: card.Ace, Suit: card.Clubs}
	queen := card.Card{Rank: card.Queen, Suit: card.Spades}

	s := GameState{
		Players: [2]PlayerState{
			{
				Permanents: []card.Card{queen},
				Points: []PointEntry{
					{Card: card.Card{Rank: card.Seven, Suit: card.Hearts}, Owner: P1},
					{Card: card.Card{Rank: card.Five, Suit: card.Clubs}, Owner: P1},
				},
			},
			{
				Hand: []card.Card{ace},
				Points: []PointEntry{
					{Card: card.Card{Rank: card.Ten, Suit: card.Diamonds}, Owner: P2},
					{Card: card.Card{Rank: card.Four, Suit: card.Spades}, Owner: P2},
				},
			},
		},
		Active: P2,
		Phase:  PhaseNormal,
	}
	padDeckTo52(&s)
	assertDeckCount(t, s, 52)

	moves := LegalMoves(s)
	playAce, ok := findMove(moves, func(m Move) bool {
		return m.Kind == MoveOneOff && m.Card == ace
	})
	if !ok {
		t.Fatal("expected MoveOneOff Ace in legal moves")
	}

	s2, err := Apply(s, playAce)
	if err != nil {
		t.Fatal(err)
	}
	// P1 has no 2s → immediate resolution.
	if s2.Phase != PhaseNormal {
		t.Errorf("expected PhaseNormal, got %v", s2.Phase)
	}
	if len(s2.Players[P1].Points) != 0 {
		t.Errorf("P1 points should be wiped, got %v", s2.Players[P1].Points)
	}
	if len(s2.Players[P2].Points) != 0 {
		t.Errorf("P2 points should be wiped, got %v", s2.Players[P2].Points)
	}
	// Queen remains on P1's permanents.
	if len(s2.Players[P1].Permanents) != 1 || s2.Players[P1].Permanents[0] != queen {
		t.Errorf("Queen should remain on P1 permanents, got %v", s2.Players[P1].Permanents)
	}
	// Scrap: ace + 7♥ + 5♣ + 10♦ + 4♠ = 5 cards.
	if len(s2.Scrap) != 5 {
		t.Errorf("expected 5 scrap cards, got %d", len(s2.Scrap))
	}
	assertDeckCount(t, s2, 52)
}

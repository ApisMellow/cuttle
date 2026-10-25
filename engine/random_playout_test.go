package engine

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/ApisMellow/cuttle/card"
)

// maxPlayoutSteps bounds a single random game. Random play terminates far
// below this in practice; hitting the cap means a livelock.
const maxPlayoutSteps = 5000

// dealRandomPlayout shuffles a full 52-card deck with rng and deals the
// standard opening: 5 cards to the dealer's opponent... in this engine's
// convention, 5 to P1 and 6 to P2, matching cmd/cuttle's newGame.
func dealRandomPlayout(rng *rand.Rand) GameState {
	deck := make([]card.Card, 0, 52)
	for s := card.Clubs; s <= card.Spades; s++ {
		for r := card.Ace; r <= card.King; r++ {
			deck = append(deck, card.Card{Rank: r, Suit: s})
		}
	}
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	return GameState{
		Players: [2]PlayerState{
			{Hand: append([]card.Card(nil), deck[:5]...)},
			{Hand: append([]card.Card(nil), deck[5:11]...)},
		},
		Deck:   append([]card.Card(nil), deck[11:]...),
		Active: P1,
		Phase:  PhaseNormal,
	}
}

// countAllCards tallies every card visible in every zone of the state.
// Pending.Card and Pending.CounterChain are live (removed from hands, not
// yet scrapped) only while a counter window is open; in PhaseSevenChoosing
// the one-off itself has already been scrapped.
// Pending.Revealed cards have been removed from the deck.
func countAllCards(s GameState) map[card.Card]int {
	counts := make(map[card.Card]int, 52)
	add := func(cs ...card.Card) {
		for _, c := range cs {
			counts[c]++
		}
	}
	add(s.Deck...)
	add(s.Scrap...)
	for i := 0; i < 2; i++ {
		p := s.Players[i]
		add(p.Hand...)
		add(p.Permanents...)
		for _, pe := range p.Points {
			add(pe.Card)
			add(pe.JackStack...)
		}
	}
	if s.Pending != nil {
		if s.Phase == PhaseAwaitingCounter {
			add(s.Pending.Card)
		}
		add(s.Pending.CounterChain...)
		add(s.Pending.Revealed...)
	}
	return counts
}

// checkPlayoutInvariants asserts properties that must hold after every Apply.
func checkPlayoutInvariants(t *testing.T, s GameState, seed int64, step int) {
	t.Helper()

	// Card conservation: all 52 cards exist exactly once across all zones.
	counts := countAllCards(s)
	if len(counts) != 52 {
		t.Fatalf("seed %d step %d: %d distinct cards, want 52", seed, step, len(counts))
	}
	for c, n := range counts {
		if n != 1 {
			t.Fatalf("seed %d step %d: card %s appears %d times", seed, step, c, n)
		}
	}

	// Pending is set exactly in the phases that require it. New play reaches
	// PhaseAwaitingDiscard only as the owed-discard step: no Pending, a debt,
	// and cards to pay it with.
	pendingPhase := s.Phase == PhaseAwaitingCounter || s.Phase == PhaseSevenChoosing
	if s.Phase == PhaseAwaitingDiscard && (s.Owed[s.Active] <= 0 || len(s.Players[s.Active].Hand) == 0) {
		t.Fatalf("seed %d step %d: owed-discard step with owed=%v hand=%v", seed, step, s.Owed, s.Players[s.Active].Hand)
	}
	if pendingPhase && s.Pending == nil {
		t.Fatalf("seed %d step %d: phase %d requires Pending, got nil", seed, step, s.Phase)
	}
	if !pendingPhase && s.Pending != nil {
		t.Fatalf("seed %d step %d: phase %d must not carry Pending", seed, step, s.Phase)
	}

	// A declared winner must actually satisfy the win condition; a
	// winnerless game-over is only reachable via the pass stalemate.
	if s.Phase == PhaseGameOver {
		if s.Winner != nil {
			if !HasWon(s.Players[*s.Winner]) {
				t.Fatalf("seed %d step %d: declared winner P%d has not won (points=%d, threshold=%d)",
					seed, step, *s.Winner+1, PointTotal(s.Players[*s.Winner]), Threshold(KingCount(s.Players[*s.Winner])))
			}
		} else if s.PassesInARow < 3 {
			t.Fatalf("seed %d step %d: game over with no winner and only %d passes", seed, step, s.PassesInARow)
		}
	} else if s.Winner != nil {
		t.Fatalf("seed %d step %d: winner set but phase is %d", seed, step, s.Phase)
	}

	// A pending win is held by the player who reached their threshold, and
	// the decider can counter it.
	if s.Phase == PhasePendingWin {
		if !HasWon(s.Players[s.Active.Other()]) || !s.LastTurn.Counterable {
			t.Fatalf("seed %d step %d: pending win for P%d, who has not won on a counterable turn", seed, step, s.Active.Other()+1)
		}
		if !hasLegalCounter(s.Players[s.Active]) {
			t.Fatalf("seed %d step %d: pending win, but P%d holds no unfrozen 2", seed, step, s.Active+1)
		}
	}

	// Counter-rewind snapshots are one level deep.
	for p, snap := range s.TurnEnd {
		if snap != nil && (snap.TurnEnd[P1] != nil || snap.TurnEnd[P2] != nil) {
			t.Fatalf("seed %d step %d: TurnEnd[%d] carries its own snapshots", seed, step, p)
		}
	}
}

// TestRandomPlayouts plays full games choosing uniformly among LegalMoves,
// asserting after every step that cards are conserved, every generated move
// is applicable, Apply never mutates its input, and games terminate with a
// coherent result.
func TestRandomPlayouts(t *testing.T) {
	const games = 500
	counters, pendingWins, owedDiscards := 0, 0, 0
	defer func() {
		// So must the owed discard (a 4's debt paid at turn start).
		if !t.Failed() && owedDiscards == 0 {
			t.Errorf("no game reached an owed discard in %d games", games)
		}
		// Counter rewinds must actually be in play, not just offered.
		if !t.Failed() && counters == 0 {
			t.Errorf("no counter was applied in %d games", games)
		}
		// So must the pending win ("Not So Fast!").
		if !t.Failed() && pendingWins == 0 {
			t.Errorf("no game reached PhasePendingWin in %d games", games)
		}
	}()
	for seed := int64(1); seed <= games; seed++ {
		rng := rand.New(rand.NewSource(seed))
		s := dealRandomPlayout(rng)
		checkPlayoutInvariants(t, s, seed, 0)
		step := 0
		for s.Phase != PhaseGameOver {
			step++
			if step > maxPlayoutSteps {
				t.Fatalf("seed %d: game did not terminate within %d steps", seed, maxPlayoutSteps)
			}
			moves := LegalMoves(s)
			if len(moves) == 0 {
				t.Fatalf("seed %d step %d: no legal moves in phase %d", seed, step, s.Phase)
			}
			// Every generated move must be applicable without error.
			for i, m := range moves {
				if _, err := Apply(s, m); err != nil {
					t.Fatalf("seed %d step %d: legal move %d (%s) rejected by Apply: %v",
						seed, step, i, m.Describe(s), err)
				}
			}
			m := moves[rng.Intn(len(moves))]
			if m.Kind == MoveCounter {
				counters++
			}
			before := clone(s)
			next, err := Apply(s, m)
			if err != nil {
				t.Fatalf("seed %d step %d: Apply(%s): %v", seed, step, m.Describe(s), err)
			}
			// Apply must not mutate its input. Compare clone-to-clone so
			// clone's nil-vs-empty slice normalization cancels out.
			if !reflect.DeepEqual(before, clone(s)) {
				t.Fatalf("seed %d step %d: Apply mutated its input state (move: %s)", seed, step, m.Describe(s))
			}
			s = next
			if s.Phase == PhasePendingWin {
				pendingWins++
			}
			if s.Phase == PhaseAwaitingDiscard {
				owedDiscards++
			}
			checkPlayoutInvariants(t, s, seed, step)
		}
	}
}

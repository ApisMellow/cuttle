package engine

import (
	"errors"

	"github.com/ApisMellow/cuttle/card"
)

var ErrIllegalMove = errors.New("illegal move")

const HandLimit = 8

// LegalMoves returns every legal move from the current state.
// The REPL displays this list and the user picks one by index.
func LegalMoves(s GameState) []Move {
	if s.Phase == PhaseGameOver {
		return nil
	}
	if s.Phase == PhaseAwaitingCounter {
		return legalCounterMoves(s)
	}
	if s.Phase == PhaseAwaitingDiscard {
		return legalDiscardMoves(s)
	}
	if s.Phase == PhaseSevenChoosing {
		return legalSevenPickMoves(s)
	}
	if s.Phase != PhaseNormal {
		// later phases handled in later tasks
		return nil
	}
	var moves []Move
	active := s.Players[s.Active]
	if len(s.Deck) > 0 && len(active.Hand) < HandLimit {
		moves = append(moves, Move{Kind: MoveDraw})
	}
	for i, c := range active.Hand {
		if active.FrozenIDs[i] {
			continue
		}
		if c.Rank == card.Ace || c.Rank == card.Six || c.Rank == card.Four || c.Rank == card.Five {
			moves = append(moves, Move{Kind: MoveOneOff, Card: c, HandIndex: i})
		}
		if c.Rank == card.Two {
			// 2-as-scrap: target one royal (Q/K), glasses-8, or Jack on
			// a point stack on either side, subject to Queen protection.
			for pl := 0; pl < 2; pl++ {
				owner := PlayerID(pl)
				hasQueen := ownerHasQueen(s.Players[owner])
				for j, perm := range s.Players[owner].Permanents {
					if hasQueen && perm.Rank != card.Queen {
						continue
					}
					tgt := Target{Owner: owner, Zone: ZonePermanents, Index: j}
					moves = append(moves, Move{Kind: MoveOneOff, Card: c, HandIndex: i, Target: &tgt})
				}
				for j, pe := range s.Players[owner].Points {
					if len(pe.JackStack) == 0 {
						continue
					}
					// Jack on point: queen on owner side protects it.
					if hasQueen {
						continue
					}
					tgt := Target{Owner: owner, Zone: ZonePoints, Index: j}
					moves = append(moves, Move{Kind: MoveOneOff, Card: c, HandIndex: i, Target: &tgt})
				}
			}
		}
		if c.Rank == card.Nine {
			opp := s.Active.Other()
			oppHasQueen := ownerHasQueen(s.Players[opp])
			for j := range s.Players[opp].Points {
				if oppHasQueen {
					continue
				}
				tgt := Target{Owner: opp, Zone: ZonePoints, Index: j}
				moves = append(moves, Move{Kind: MoveOneOff, Card: c, HandIndex: i, Target: &tgt})
			}
			for j, perm := range s.Players[opp].Permanents {
				if oppHasQueen && perm.Rank != card.Queen {
					continue
				}
				tgt := Target{Owner: opp, Zone: ZonePermanents, Index: j}
				moves = append(moves, Move{Kind: MoveOneOff, Card: c, HandIndex: i, Target: &tgt})
			}
		}
		if c.Rank == card.Jack {
			opp := s.Active.Other()
			oppHasQueen := ownerHasQueen(s.Players[opp])
			if !oppHasQueen {
				for j := range s.Players[opp].Points {
					tgt := Target{Owner: opp, Zone: ZonePoints, Index: j}
					moves = append(moves, Move{Kind: MovePlayPermanent, Card: c, HandIndex: i, JackTarget: &tgt})
				}
			}
		}
		if c.Rank == card.Three {
			// Three: take any one card from scrap into hand. One move per
			// scrap card. No legal play if scrap is empty. Playing the 3
			// removes it (-1) and takes a scrap card (+1), so the
			// post-resolution hand size equals the pre-play hand size;
			// we gate on that not exceeding HandLimit.
			if len(active.Hand) <= HandLimit {
				for si := range s.Scrap {
					moves = append(moves, Move{
						Kind: MoveOneOff, Card: c, HandIndex: i, ScrapIndex: si,
					})
				}
			}
		}
		if c.Rank >= card.Ace && c.Rank <= card.Ten {
			moves = append(moves, Move{Kind: MovePlayPoint, Card: c, HandIndex: i})
			opp := s.Active.Other()
			for j, pe := range s.Players[opp].Points {
				if c.Beats(pe.Card) {
					moves = append(moves, Move{
						Kind: MoveScuttle, Card: c, HandIndex: i,
						Target: &Target{Owner: opp, Zone: ZonePoints, Index: j},
					})
				}
			}
		}
		if c.Rank == card.Seven && len(s.Deck) > 0 {
			moves = append(moves, Move{Kind: MoveOneOff, Card: c, HandIndex: i})
		}
		if c.Rank == card.Queen || c.Rank == card.King || c.Rank == card.Eight {
			moves = append(moves, Move{Kind: MovePlayPermanent, Card: c, HandIndex: i})
		}
	}
	if len(moves) == 0 {
		moves = append(moves, Move{Kind: MovePass})
	}
	// Counters go last: cuttle-web's replay scenarios index into this slice,
	// so the moves above must keep their positions.
	if counterAvailable(s) {
		for i, c := range active.Hand {
			if c.Rank == card.Two && !active.FrozenIDs[i] {
				moves = append(moves, Move{Kind: MoveCounter, Card: c, HandIndex: i})
			}
		}
	}
	return moves
}

// counterAvailable reports whether the active player may open their turn in
// PhaseNormal with a counter: the opponent's last turn was counterable, no
// move has been made this turn yet, and there is a position to restore (the
// end of the active player's own last turn; absent on their first turn).
func counterAvailable(s GameState) bool {
	return s.Phase == PhaseNormal && s.LastTurn.Counterable &&
		!s.ThisTurn.Counterable && s.TurnEnd[s.Active] != nil
}

// Apply validates the move (must appear in LegalMoves(s)) and returns the new state.
func Apply(s GameState, m Move) (GameState, error) {
	return apply(s, m, false)
}

// apply is Apply; fromSeven is true for a 7's pick, whose card came from the
// deck rather than the hand and so is not a countered card.
func apply(s GameState, m Move, fromSeven bool) (GameState, error) {
	if s.Phase == PhaseGameOver {
		return s, ErrIllegalMove
	}
	out := clone(s)
	switch m.Kind {
	case MoveDraw:
		if len(out.Deck) == 0 || len(out.Players[out.Active].Hand) >= HandLimit {
			return s, ErrIllegalMove
		}
		top := out.Deck[0]
		out.Deck = out.Deck[1:]
		out.Players[out.Active].Hand = append(out.Players[out.Active].Hand, top)
		out.PassesInARow = 0
		endTurn(&out)
		return out, nil
	case MovePass:
		// Pass is only legal when the active player has no other action.
		// LegalMoves emits MovePass solely as a fallback when no real move
		// is available (apply.go LegalMoves: `if len(moves) == 0`); Apply
		// enforces the same gate so callers cannot march toward stalemate
		// while real moves remain.
		if s.Phase == PhaseNormal {
			for _, lm := range LegalMoves(s) {
				if lm.Kind != MovePass {
					return s, ErrIllegalMove
				}
			}
		}
		out.PassesInARow++
		if out.PassesInARow >= 3 {
			out.Phase = PhaseGameOver
			return out, nil
		}
		endTurn(&out)
		return out, nil
	case MovePlayPoint:
		p := &out.Players[out.Active]
		if m.HandIndex < 0 || m.HandIndex >= len(p.Hand) || p.Hand[m.HandIndex] != m.Card {
			return s, ErrIllegalMove
		}
		if p.FrozenIDs[m.HandIndex] {
			return s, ErrIllegalMove
		}
		if m.Card.Rank < card.Ace || m.Card.Rank > card.Ten {
			return s, ErrIllegalMove
		}
		removeFromHand(p, m.HandIndex)
		p.Points = append(p.Points, PointEntry{Card: m.Card, Owner: out.Active})
		out.PassesInARow = 0
		if checkWin(&out, out.Active) {
			return out, nil
		}
		endTurn(&out)
		return out, nil
	case MovePlayPermanent:
		p := &out.Players[out.Active]
		if m.HandIndex < 0 || m.HandIndex >= len(p.Hand) || p.Hand[m.HandIndex] != m.Card {
			return s, ErrIllegalMove
		}
		if p.FrozenIDs[m.HandIndex] {
			return s, ErrIllegalMove
		}
		if m.Card.Rank == card.Jack {
			// Jack-as-permanent: steal an opponent point by transplanting
			// the PointEntry to the attacker's Points slice. Subject to
			// Queen protection.
			if m.JackTarget == nil || m.JackTarget.Zone != ZonePoints {
				return s, ErrIllegalMove
			}
			victim := m.JackTarget.Owner
			if victim == out.Active {
				return s, ErrIllegalMove
			}
			vp := &out.Players[victim]
			if m.JackTarget.Index < 0 || m.JackTarget.Index >= len(vp.Points) {
				return s, ErrIllegalMove
			}
			if ownerHasQueen(*vp) {
				return s, ErrIllegalMove
			}
			pe := vp.Points[m.JackTarget.Index]
			vp.Points = removeAt(vp.Points, m.JackTarget.Index)
			pe.JackStack = append(pe.JackStack, m.Card)
			pe.JackOwners = append(pe.JackOwners, out.Active)
			removeFromHand(p, m.HandIndex)
			out.Players[out.Active].Points = append(out.Players[out.Active].Points, pe)
			out.PassesInARow = 0
			if checkWin(&out, out.Active) {
				return out, nil
			}
			endTurn(&out)
			return out, nil
		}
		if m.Card.Rank != card.Queen && m.Card.Rank != card.King && m.Card.Rank != card.Eight {
			return s, ErrIllegalMove
		}
		removeFromHand(p, m.HandIndex)
		p.Permanents = append(p.Permanents, m.Card)
		out.PassesInARow = 0
		if checkWin(&out, out.Active) {
			return out, nil
		}
		endTurn(&out)
		return out, nil
	case MoveScuttle:
		p := &out.Players[out.Active]
		if m.HandIndex < 0 || m.HandIndex >= len(p.Hand) || p.Hand[m.HandIndex] != m.Card {
			return s, ErrIllegalMove
		}
		if p.FrozenIDs[m.HandIndex] {
			return s, ErrIllegalMove
		}
		if m.Target == nil || m.Target.Zone != ZonePoints {
			return s, ErrIllegalMove
		}
		opp := &out.Players[m.Target.Owner]
		if m.Target.Index < 0 || m.Target.Index >= len(opp.Points) {
			return s, ErrIllegalMove
		}
		target := opp.Points[m.Target.Index]
		if !m.Card.Beats(target.Card) {
			return s, ErrIllegalMove
		}
		removeFromHand(p, m.HandIndex)
		out.Scrap = append(out.Scrap, m.Card, target.Card)
		out.Scrap = append(out.Scrap, target.JackStack...)
		opp.Points = removeAt(opp.Points, m.Target.Index)
		out.PassesInARow = 0
		endTurn(&out)
		return out, nil
	case MoveOneOff:
		p := &out.Players[out.Active]
		if m.HandIndex < 0 || m.HandIndex >= len(p.Hand) || p.Hand[m.HandIndex] != m.Card {
			return s, ErrIllegalMove
		}
		if p.FrozenIDs[m.HandIndex] {
			return s, ErrIllegalMove
		}
		if m.Card.Rank != card.Ace && m.Card.Rank != card.Six && m.Card.Rank != card.Three && m.Card.Rank != card.Four && m.Card.Rank != card.Five && m.Card.Rank != card.Nine && m.Card.Rank != card.Two && m.Card.Rank != card.Seven {
			return s, ErrIllegalMove
		}
		if m.Card.Rank == card.Three {
			if m.ScrapIndex < 0 || m.ScrapIndex >= len(out.Scrap) {
				return s, ErrIllegalMove
			}
		}
		if m.Card.Rank == card.Two {
			if m.Target == nil {
				return s, ErrIllegalMove
			}
			tp := &out.Players[m.Target.Owner]
			switch m.Target.Zone {
			case ZonePermanents:
				if m.Target.Index < 0 || m.Target.Index >= len(tp.Permanents) {
					return s, ErrIllegalMove
				}
			case ZonePoints:
				if m.Target.Index < 0 || m.Target.Index >= len(tp.Points) {
					return s, ErrIllegalMove
				}
				if len(tp.Points[m.Target.Index].JackStack) == 0 {
					return s, ErrIllegalMove
				}
			default:
				return s, ErrIllegalMove
			}
		}
		if m.Card.Rank == card.Nine {
			if m.Target == nil {
				return s, ErrIllegalMove
			}
			tp := &out.Players[m.Target.Owner]
			switch m.Target.Zone {
			case ZonePoints:
				if m.Target.Index < 0 || m.Target.Index >= len(tp.Points) {
					return s, ErrIllegalMove
				}
			case ZonePermanents:
				if m.Target.Index < 0 || m.Target.Index >= len(tp.Permanents) {
					return s, ErrIllegalMove
				}
			default:
				return s, ErrIllegalMove
			}
		}
		played := out.Active
		removeFromHand(p, m.HandIndex)
		out.PassesInARow = 0
		// A one-off takes effect at once; the opponent may counter it at the
		// start of their turn (counter rewind). New play never enters
		// PhaseAwaitingCounter.
		out.ThisTurn.Counterable = true
		if !fromSeven {
			out.ThisTurn.Countered = append(out.ThisTurn.Countered, m.Card)
		}
		resolveOneOffWith(&out, m.Card, played, m.ScrapIndex, m.Target)
		return out, nil
	case MoveCounter:
		if s.Phase == PhaseNormal {
			return applyRewind(s, m)
		}
		// Legacy: a v0.2.x state saved mid-window.
		if s.Phase != PhaseAwaitingCounter || out.Pending == nil {
			return s, ErrIllegalMove
		}
		p := &out.Players[out.Active]
		if m.HandIndex < 0 || m.HandIndex >= len(p.Hand) || p.Hand[m.HandIndex] != m.Card {
			return s, ErrIllegalMove
		}
		if m.Card.Rank != card.Two {
			return s, ErrIllegalMove
		}
		if p.FrozenIDs[m.HandIndex] {
			return s, ErrIllegalMove
		}
		removeFromHand(p, m.HandIndex)
		out.Pending.CounterChain = append(out.Pending.CounterChain, m.Card)
		// Flip waiting player; if they can counter, stay in phase; else auto-resolve.
		next := out.Active.Other()
		if hasLegalCounter(out.Players[next]) {
			out.Active = next
			return out, nil
		}
		resolvePending(&out)
		return out, nil
	case MoveDiscardPair:
		if s.Phase != PhaseAwaitingDiscard || out.Pending == nil {
			return s, ErrIllegalMove
		}
		p := &out.Players[out.Active]
		n := len(p.Hand)
		if n == 0 {
			return s, ErrIllegalMove
		}
		a, b := m.DiscardA, m.DiscardB
		if n == 1 {
			if a != 0 || b != -1 {
				return s, ErrIllegalMove
			}
			out.Scrap = append(out.Scrap, p.Hand[0])
			removeFromHand(p, 0)
		} else {
			if a < 0 || b < 0 || a >= n || b >= n || a == b {
				return s, ErrIllegalMove
			}
			if a > b {
				a, b = b, a
			}
			// Remove higher index first so lower index remains valid.
			ca, cb := p.Hand[a], p.Hand[b]
			removeFromHand(p, b)
			removeFromHand(p, a)
			out.Scrap = append(out.Scrap, ca, cb)
		}
		played := out.Pending.PlayedBy
		out.Pending = nil
		out.Phase = PhaseNormal
		out.Active = played
		endTurn(&out)
		return out, nil
	case MoveSevenPick:
		if s.Phase != PhaseSevenChoosing || out.Pending == nil {
			return s, ErrIllegalMove
		}
		if m.SubMove != nil && (m.SubMove.Kind == MoveDraw || m.SubMove.Kind == MovePass) {
			return s, ErrIllegalMove
		}
		rev := out.Pending.Revealed
		chosenIdx := -1
		for i, c := range rev {
			if c == m.Card {
				chosenIdx = i
				break
			}
		}
		if chosenIdx < 0 {
			return s, ErrIllegalMove
		}
		if m.SubMove == nil {
			// Dead-end scrap: when NO revealed card has any legal play, the
			// player scraps one revealed card and any other returns to the
			// top of the deck. Only legal in that dead-end state.
			for _, c := range rev {
				if len(legalForCard(out, c)) > 0 {
					return s, ErrIllegalMove
				}
			}
			played := out.Pending.PlayedBy
			var unchosen []card.Card
			for i, c := range rev {
				if i != chosenIdx {
					unchosen = append(unchosen, c)
				}
			}
			if len(unchosen) > 0 {
				out.Deck = append(append([]card.Card(nil), unchosen...), out.Deck...)
			}
			out.Scrap = append(out.Scrap, m.Card)
			out.Pending = nil
			out.Phase = PhaseNormal
			out.Active = played
			endTurn(&out)
			return out, nil
		}
		played := out.Pending.PlayedBy
		// Push unchosen (if any) back to top of deck.
		var unchosen []card.Card
		for i, c := range rev {
			if i == chosenIdx {
				continue
			}
			unchosen = append(unchosen, c)
		}
		if len(unchosen) > 0 {
			out.Deck = append(append([]card.Card(nil), unchosen...), out.Deck...)
		}
		out.Pending = nil
		out.Phase = PhaseNormal
		out.Active = played
		// Inject chosen card into the active player's hand at a known index,
		// then dispatch the inner move with that HandIndex.
		p := &out.Players[played]
		injectIdx := len(p.Hand)
		p.Hand = append(p.Hand, m.Card)
		sub := *m.SubMove
		sub.Card = m.Card
		sub.HandIndex = injectIdx
		return apply(out, sub, true)
	case MoveDecline:
		if s.Phase != PhaseAwaitingCounter || out.Pending == nil {
			return s, ErrIllegalMove
		}
		resolvePending(&out)
		return out, nil
	}
	return s, ErrIllegalMove
}

// applyRewind plays a counter as the first action of the active player's
// turn. The game goes back to TurnEnd[counterer] (the end of the counterer's
// own last turn, which is the start of the opponent's erased turn). Each
// countered card still in the opponent's hand there goes to the scrap, in
// play order, then the counterer's 2 on top. All frozen marks are cleared,
// and the counterer goes on to take a normal turn, which is counterable
// because it began with a counter. TurnEnd is left untouched.
func applyRewind(s GameState, m Move) (GameState, error) {
	if !counterAvailable(s) || m.Card.Rank != card.Two {
		return s, ErrIllegalMove
	}
	cp := s.Players[s.Active]
	if m.HandIndex < 0 || m.HandIndex >= len(cp.Hand) || cp.Hand[m.HandIndex] != m.Card {
		return s, ErrIllegalMove
	}
	if cp.FrozenIDs[m.HandIndex] {
		return s, ErrIllegalMove
	}
	counterer := s.Active
	opp := counterer.Other()
	out := clone(*s.TurnEnd[counterer])
	// The counterer's 2 is always in their hand at the snapshot: the only 2s
	// that reach a hand during the opponent's turn come back through a 9,
	// and those are frozen.
	if handIndexOf(out.Players[counterer].Hand, m.Card) < 0 {
		return s, ErrIllegalMove
	}
	for _, c := range s.LastTurn.Countered {
		if i := handIndexOf(out.Players[opp].Hand, c); i >= 0 {
			removeFromHand(&out.Players[opp], i)
			out.Scrap = append(out.Scrap, c)
		}
	}
	removeFromHand(&out.Players[counterer], handIndexOf(out.Players[counterer].Hand, m.Card))
	out.Scrap = append(out.Scrap, m.Card)
	out.Players[P1].FrozenIDs = nil
	out.Players[P2].FrozenIDs = nil
	out.PassesInARow = 0
	out.Active = counterer
	out.Phase = PhaseNormal
	out.Pending = nil
	out.Winner = nil
	out.TurnEnd = s.TurnEnd
	// The opponent's turn is erased, so there is nothing left to counter.
	out.LastTurn = TurnRecord{}
	out.ThisTurn = TurnRecord{Counterable: true, Countered: []card.Card{m.Card}}
	return out, nil
}

func handIndexOf(hand []card.Card, c card.Card) int {
	for i, h := range hand {
		if h == c {
			return i
		}
	}
	return -1
}

// ownerHasQueen reports whether p has at least one Queen permanent.
func ownerHasQueen(p PlayerState) bool {
	for _, c := range p.Permanents {
		if c.Rank == card.Queen {
			return true
		}
	}
	return false
}

// hasLegalCounter reports whether p has any non-frozen 2 in hand.
func hasLegalCounter(p PlayerState) bool {
	for i, c := range p.Hand {
		if c.Rank == card.Two && !p.FrozenIDs[i] {
			return true
		}
	}
	return false
}

// legalDiscardMoves lists every unordered pair of hand indices for the
// discarding (active) player. If hand has exactly 1 card, emits a single
// move with DiscardA=0, DiscardB=-1. If hand is empty, returns nil (the
// Four handler auto-resumes before entering this phase).
func legalDiscardMoves(s GameState) []Move {
	p := s.Players[s.Active]
	n := len(p.Hand)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return []Move{{Kind: MoveDiscardPair, DiscardA: 0, DiscardB: -1}}
	}
	var moves []Move
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			moves = append(moves, Move{Kind: MoveDiscardPair, DiscardA: i, DiscardB: j})
		}
	}
	return moves
}

// legalSevenPickMoves wraps each legal inner move for each revealed card as
// MoveSevenPick. Draw and Pass are excluded. Frozen/queen rules are
// inherited automatically via the reused LegalMoves enumeration.
func legalSevenPickMoves(s GameState) []Move {
	if s.Pending == nil {
		return nil
	}
	var out []Move
	for _, c := range s.Pending.Revealed {
		for _, inner := range legalForCard(s, c) {
			sub := inner
			out = append(out, Move{Kind: MoveSevenPick, Card: c, SubMove: &sub})
		}
	}
	if len(out) == 0 {
		// Dead end: no revealed card has any legal play. The player must
		// scrap one revealed card (SubMove nil); any other returns to the
		// top of the deck.
		for _, c := range s.Pending.Revealed {
			out = append(out, Move{Kind: MoveSevenPick, Card: c})
		}
	}
	return out
}

// legalForCard returns the legal moves the given card could make if it were
// the only playable card in the active player's hand (for Seven resolution).
// Draw and Pass are excluded.
func legalForCard(s GameState, c card.Card) []Move {
	faux := clone(s)
	faux.Phase = PhaseNormal
	faux.Pending = nil
	p := &faux.Players[faux.Active]
	// Pretend the active player has ONLY this card. Frozen marks are
	// cleared for the injected position; existing field state is preserved.
	p.Hand = []card.Card{c}
	p.FrozenIDs = nil
	// Also blank the deck presence isn't affected here; Seven's own deck
	// reveal already mutated s.Deck. For inner enumeration we care about
	// whether a 7 sub-pick can itself reveal — but we simply allow it if
	// faux deck has cards, matching the real Seven-in-hand rule.
	all := LegalMoves(faux)
	var out []Move
	for _, m := range all {
		if m.Kind == MoveDraw || m.Kind == MovePass {
			continue
		}
		if m.HandIndex != 0 {
			continue
		}
		out = append(out, m)
	}
	return out
}

// legalCounterMoves lists decline + one MoveCounter per legal 2.
func legalCounterMoves(s GameState) []Move {
	moves := []Move{{Kind: MoveDecline}}
	p := s.Players[s.Active]
	for i, c := range p.Hand {
		if c.Rank == card.Two && !p.FrozenIDs[i] {
			moves = append(moves, Move{Kind: MoveCounter, Card: c, HandIndex: i})
		}
	}
	return moves
}

// resolvePending finalizes a counter chain: even len → resolve, odd → cancel.
// Either way, the original card and every 2 in the chain go to scrap, then
// control returns to the original player and endTurn runs.
func resolvePending(s *GameState) {
	pend := s.Pending
	s.Pending = nil
	s.Phase = PhaseNormal
	s.Active = pend.PlayedBy
	cancelled := len(pend.CounterChain)%2 == 1
	if cancelled {
		// Card is cancelled; just scrap original + chain.
		s.Scrap = append(s.Scrap, pend.Card)
		s.Scrap = append(s.Scrap, pend.CounterChain...)
		endTurn(s)
		return
	}
	// Resolve the original one-off's effect. The chain 2s go to scrap alongside.
	resolveOneOffWith(s, pend.Card, pend.PlayedBy, pend.ScrapIndex, pend.Target)
	// Append the chain 2s to scrap (resolveOneOffWith already scrapped the original + effect).
	s.Scrap = append(s.Scrap, pend.CounterChain...)
}

// resolveOneOffWith applies the effect of a one-off card played by `played`
// and scraps the card itself, runs win check, and ends the turn.
func resolveOneOffWith(s *GameState, c card.Card, played PlayerID, scrapIndex int, target *Target) {
	s.Active = played
	// Three: take the chosen scrap card into the played-by player's hand
	// BEFORE appending the 3 to scrap (so ScrapIndex still refers to the
	// scrap pile as it existed when the move was chosen).
	if c.Rank == card.Three {
		if scrapIndex >= 0 && scrapIndex < len(s.Scrap) {
			taken := s.Scrap[scrapIndex]
			s.Scrap = removeAt(s.Scrap, scrapIndex)
			s.Players[played].Hand = append(s.Players[played].Hand, taken)
		}
	}
	s.Scrap = append(s.Scrap, c)
	switch c.Rank {
	case card.Ace:
		for i := 0; i < 2; i++ {
			pl := &s.Players[i]
			for _, pe := range pl.Points {
				s.Scrap = append(s.Scrap, pe.Card)
				s.Scrap = append(s.Scrap, pe.JackStack...)
			}
			pl.Points = nil
		}
	case card.Five:
		// Draw up to 2 cards, respecting 8-card hand limit and deck size.
		pl := &s.Players[played]
		for i := 0; i < 2; i++ {
			if len(s.Deck) == 0 || len(pl.Hand) >= HandLimit {
				break
			}
			pl.Hand = append(pl.Hand, s.Deck[0])
			s.Deck = s.Deck[1:]
		}
	case card.Four:
		// Opponent discards 2 cards (or 1 if hand has 1, or 0 auto-resume).
		opp := played.Other()
		if len(s.Players[opp].Hand) == 0 {
			// Auto-resume: no discard needed, end turn normally.
			break
		}
		s.Phase = PhaseAwaitingDiscard
		s.Active = opp
		s.Pending = &PendingOneOff{PlayedBy: played, Card: c}
		return
	case card.Six:
		// Scrap all royals and glasses-8s on both sides. Permanents
		// slice only holds Q/K/glasses-8 (Jacks live in point stacks),
		// so wiping it scraps every non-Jack royal. Jacks are then
		// stripped from point stacks; each stripped point returns to
		// its original Owner *before* its Jacks hit the scrap pile.
		for i := 0; i < 2; i++ {
			pl := &s.Players[i]
			s.Scrap = append(s.Scrap, pl.Permanents...)
			pl.Permanents = nil
		}
		var kept [2][]PointEntry
		for i := 0; i < 2; i++ {
			for _, pe := range s.Players[i].Points {
				if len(pe.JackStack) == 0 {
					kept[i] = append(kept[i], pe)
					continue
				}
				s.Scrap = append(s.Scrap, pe.JackStack...)
				pe.JackStack = nil
				pe.JackOwners = nil
				kept[pe.Owner] = append(kept[pe.Owner], pe)
			}
		}
		for i := 0; i < 2; i++ {
			s.Players[i].Points = kept[i]
		}
	case card.Two:
		if target == nil {
			break
		}
		tp := &s.Players[target.Owner]
		switch target.Zone {
		case ZonePermanents:
			if target.Index < 0 || target.Index >= len(tp.Permanents) {
				break
			}
			scrapped := tp.Permanents[target.Index]
			tp.Permanents = removeAt(tp.Permanents, target.Index)
			s.Scrap = append(s.Scrap, scrapped)
		case ZonePoints:
			if target.Index < 0 || target.Index >= len(tp.Points) {
				break
			}
			pe := tp.Points[target.Index]
			if len(pe.JackStack) == 0 {
				break
			}
			// Pop only the top Jack; buried Jacks stay. Control passes to
			// the next Jack's owner, or to the original Owner if the stack
			// is now empty, and the entry moves to that player's side so
			// scoring follows it. The win check below covers the player
			// who played the 2; if the pop takes the opponent to their
			// threshold, they win at the start of their turn (endTurn).
			top := pe.JackStack[len(pe.JackStack)-1]
			pe.JackStack = pe.JackStack[:len(pe.JackStack)-1]
			pe.JackOwners = pe.JackOwners[:len(pe.JackOwners)-1]
			s.Scrap = append(s.Scrap, top)
			if ctrl := pe.Controller(); ctrl != target.Owner {
				tp.Points = removeAt(tp.Points, target.Index)
				s.Players[ctrl].Points = append(s.Players[ctrl].Points, pe)
			} else {
				tp.Points[target.Index] = pe
			}
		}
	case card.Seven:
		// Reveal up to top 2 deck cards into Pending.Revealed and enter
		// PhaseSevenChoosing. The Seven itself has already been scrapped.
		n := 2
		if len(s.Deck) < n {
			n = len(s.Deck)
		}
		revealed := append([]card.Card(nil), s.Deck[:n]...)
		s.Deck = s.Deck[n:]
		s.Pending = &PendingOneOff{PlayedBy: played, Card: c, Revealed: revealed}
		s.Phase = PhaseSevenChoosing
		s.Active = played
		return
	case card.Nine:
		if target == nil {
			break
		}
		tp := &s.Players[target.Owner]
		var returned card.Card
		var returnTo PlayerID
		switch target.Zone {
		case ZonePoints:
			if target.Index < 0 || target.Index >= len(tp.Points) {
				break
			}
			pe := tp.Points[target.Index]
			tp.Points = removeAt(tp.Points, target.Index)
			// Scrap any Jacks on the stack; point returns to its original Owner.
			if len(pe.JackStack) > 0 {
				s.Scrap = append(s.Scrap, pe.JackStack...)
			}
			returned = pe.Card
			returnTo = pe.Owner
		case ZonePermanents:
			if target.Index < 0 || target.Index >= len(tp.Permanents) {
				break
			}
			returned = tp.Permanents[target.Index]
			tp.Permanents = removeAt(tp.Permanents, target.Index)
			returnTo = target.Owner
		}
		// Return the card, then end the turn, then freeze it: endTurn's
		// snapshot must hold the card in hand, and its frozen-clear must not
		// wipe the mark, which lasts for the turn starting now and no other.
		rp := &s.Players[returnTo]
		rp.Hand = append(rp.Hand, returned)
		frozenIdx := len(rp.Hand) - 1
		endTurn(s)
		if rp.FrozenIDs == nil {
			rp.FrozenIDs = map[int]bool{}
		}
		rp.FrozenIDs[frozenIdx] = true
		return
	}
	if checkWin(s, played) {
		return
	}
	endTurn(s)
}

// removeFromHand removes hand index i and remaps FrozenIDs so each frozen
// mark stays on the card it froze: the removed card's mark disappears, and
// marks above i shift down by one. Without this, any removal below a frozen
// index leaves the mark on the wrong card (e.g. playing a 7 and then having
// the revealed card injected at a stale frozen index).
func removeFromHand(p *PlayerState, i int) {
	p.Hand = removeAt(p.Hand, i)
	if p.FrozenIDs == nil {
		return
	}
	remapped := make(map[int]bool, len(p.FrozenIDs))
	for k, v := range p.FrozenIDs {
		switch {
		case k == i:
			// the removed card's freeze goes with it
		case k > i:
			remapped[k-1] = v
		default:
			remapped[k] = v
		}
	}
	if len(remapped) == 0 {
		p.FrozenIDs = nil
		return
	}
	p.FrozenIDs = remapped
}

func removeAt[T any](xs []T, i int) []T {
	out := make([]T, 0, len(xs)-1)
	out = append(out, xs[:i]...)
	out = append(out, xs[i+1:]...)
	return out
}

// checkWin sets phase/winner if p has reached threshold. Returns true if game ended.
func checkWin(s *GameState, p PlayerID) bool {
	if HasWon(s.Players[p]) {
		s.Phase = PhaseGameOver
		winner := p
		s.Winner = &winner
		return true
	}
	return false
}

// endTurn closes the outgoing player's turn and starts the next one:
//  1. ThisTurn moves into LastTurn.
//  2. Active hands over.
//  3. The freeze rule: both players' frozen marks are cleared (a 9 sets its
//     new mark after endTurn returns).
//  4. The snapshot TurnEnd[outgoing] is taken, one level deep.
//  5. The start-of-turn win check. A player can reach their threshold during
//     the opponent's turn (the opponent pops their own Jack with a 2, or
//     plays a 6); there is no off-turn win, so that player wins here, as
//     their turn begins and before LegalMoves offers them anything.
//
// endTurn is the only place a turn begins. A legacy counter window flips
// Active without calling it, so this never fires mid-chain; it runs once the
// chain has resolved and the turn actually passes.
func endTurn(s *GameState) {
	outgoing := s.Active
	s.LastTurn = s.ThisTurn
	s.ThisTurn = TurnRecord{}
	s.Active = outgoing.Other()
	s.Players[P1].FrozenIDs = nil
	s.Players[P2].FrozenIDs = nil
	snap := clone(*s)
	snap.TurnEnd = [2]*GameState{}
	s.TurnEnd[outgoing] = &snap
	checkWin(s, s.Active)
}

func clone(s GameState) GameState {
	out := s
	for i := 0; i < 2; i++ {
		out.Players[i] = clonePlayer(s.Players[i])
	}
	out.Deck = append([]card.Card(nil), s.Deck...)
	out.Scrap = append([]card.Card(nil), s.Scrap...)
	if s.Pending != nil {
		p := *s.Pending
		p.Revealed = append([]card.Card(nil), s.Pending.Revealed...)
		p.CounterChain = append([]card.Card(nil), s.Pending.CounterChain...)
		if s.Pending.Target != nil {
			tgt := *s.Pending.Target
			p.Target = &tgt
		}
		out.Pending = &p
	}
	if s.Winner != nil {
		w := *s.Winner
		out.Winner = &w
	}
	// TurnEnd snapshots are never mutated after endTurn takes them, so the
	// pointers are shared rather than copied.
	out.LastTurn.Countered = append([]card.Card(nil), s.LastTurn.Countered...)
	out.ThisTurn.Countered = append([]card.Card(nil), s.ThisTurn.Countered...)
	return out
}

func clonePlayer(p PlayerState) PlayerState {
	out := p
	out.Hand = append([]card.Card(nil), p.Hand...)
	out.Permanents = append([]card.Card(nil), p.Permanents...)
	out.Points = make([]PointEntry, len(p.Points))
	for i, pe := range p.Points {
		out.Points[i] = pe
		out.Points[i].JackStack = append([]card.Card(nil), pe.JackStack...)
		out.Points[i].JackOwners = append([]PlayerID(nil), pe.JackOwners...)
	}
	if p.FrozenIDs != nil {
		out.FrozenIDs = make(map[int]bool, len(p.FrozenIDs))
		for k, v := range p.FrozenIDs {
			out.FrozenIDs[k] = v
		}
	}
	return out
}

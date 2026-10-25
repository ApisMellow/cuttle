# Cuttle — Rules (as implemented)

Two-player card game played with a standard 52-card deck.

## Setup
- Standard 52-card deck, shuffled.
- One player is dealt 6 cards, the other is dealt 5 and goes first.
- A shared **scrap pile** sits between the players.
- Each player has their own field with two zones: **point cards** and **permanents**.

## Win Condition
- A player wins when they have **21 or more points** on their side of the field at the end of their own action, or at the start of their turn.
- You never win during your opponent's turn. If you reach your target during your opponent's turn, you win at the start of your next turn.
- A win on a turn your opponent could still **counter** (see *Countering* below) waits for them: see *Win timing*.
- **Kings** lower your own threshold: 1 King → 14, 2 Kings → 10, 3 Kings → 7, 4 Kings → 5.

## Hand Limit
- Maximum hand size: **8 cards**. You cannot draw past it.

## Turn Structure
On your turn, take exactly one action. If the deck is empty and you cannot otherwise act, you pass. Three consecutive passes = stalemate.

Your turn starts with these steps, in order:
1. **A pending win.** If your opponent reached their target on a turn you could counter, you either counter or let it happen (*Win timing*).
2. **Your own win.** If points reached you during your opponent's turn, you win now.
3. **Counter or pay.** You may counter your opponent's last turn (*Countering*). If you owe discards from a 4, you pay them now unless you counter (*The 4*).
4. **Your action.**

### Legal Actions
1. **Draw** one card from the deck.
2. Play a number card (A–10) as a **point card** on your side.
3. **Scuttle**: play a number card from hand onto an opponent's point card of strictly lower rank, OR equal rank with higher suit. Both cards go to scrap. Suit order (low → high): ♣ < ♦ < ♥ < ♠.
4. Play a **permanent** (Jack, Queen, King, or any 8 played as "glasses").
5. Play a **one-off** (A, 2, 3, 4, 5, 6, 7, 9). Resolve its effect, then scrap it.

## Card Effects

### Point Cards
- **A–10** may be played as point cards. Ace = 1, face value otherwise. 10 is point-only (no one-off).
- **8** may be played as a point card (8 points) OR as a permanent (glasses).

### One-Offs
| Card | Effect |
|---|---|
| **A** | Scrap all point cards on both sides of the field. |
| **2** | Scrap a target royal or glasses-8; OR, as the first thing you do on your turn, **counter** your opponent's last turn if it was counterable (see *Countering*). **2s can counter 2s.** |
| **3** | Take any one card from the scrap pile into your hand. |
| **4** | Your opponent owes 2 discards. Nothing is discarded now: your opponent **discards at the start of their turn** (2 cards of their choice from hand to scrap, or whatever they have if fewer), unless they counter the 4. See *The 4*. |
| **5** | Draw 2 cards and add them to your hand (respecting the 8-card limit). |
| **6** | Scrap all royals and glasses-8s on both sides. |
| **7** | Reveal the top 2 cards of the deck. Play one immediately (as point, scuttle, permanent, or one-off — normal restrictions apply). Return the other to the top of the deck. If neither revealed card has a legal play, choose one to scrap; the other returns to the top of the deck. |
| **9** | Return an opponent's field card (point or permanent) to its **original owner's** hand. Any Jacks on a returned point card are scrapped. The returned card is frozen: its owner cannot play it during the turn that immediately follows the 9, and only that turn (see *Freeze*). (If you return your own stolen point card, that turn is your opponent's, so the freeze has no effect.) |

### Permanents
| Card | Effect |
|---|---|
| **8 (glasses)** | While in play, your opponent's hand is visible to you. Any 8 may be played this way. |
| **J** | Played on top of an opponent's point card to **steal** it onto your side (it now counts for you). A Jack may target a point card already under another Jack (chain-steal); Jacks stack, and each one flips control. |
| **Q** | Your **other** cards cannot be targeted by your opponent's cards. The Queen does not protect itself, does not block board-wipes (Ace, Six), and does not block scuttling. **A Queen does block Jacks**: a protected point card cannot be stolen. |
| **K** | Lowers your win threshold. Stacks: 1K=14, 2K=10, 3K=7, 4K=5. |

## Countering
- **One-off timing**: a one-off takes effect at once and is then scrapped, and that ends your turn (a 5's two cards go straight into your hand). Nobody is asked anything in between: play simply passes to your opponent.
- **Counterable turns**: a turn is counterable if the player played a one-off from their hand during it (A, a 2 as scrap, 3, 4, 5, 6, 7 or 9), or began it with a counter.
- **The counter**: at the start of your turn, if your opponent's last turn was counterable, you may play an unfrozen 2 from your hand as your first action. The game goes back to the position at the start of their last turn, which is the end of your own last turn. Your 2 and the countered cards go to the scrap. Then you take a normal turn. Any other first action lets their turn stand for good.
- **The countered cards** are the one-off and the counter 2 that your opponent played from their hand in that turn. Each goes to the scrap if it is in their hand in the restored position; otherwise it stays where that position has it. Every other card played in the erased turn simply goes back where it was: cards drawn go back on top of the deck in their original order, a 3's card goes back to its place in the scrap, a 9's card goes back on the board.
- **A 7** is countered as a whole turn: the 7, the reveal, the pick and everything the pick did. Only the 7 is scrapped; a one-off it picked came from the deck, so it goes back there.
- **The opening turn**: the second player may counter the first player's opening one-off. The game goes back to the deal.
- **2 vs. 2 (the chain)**: counters chain, one per turn, with no pause. Alice plays a one-off. Blake counters: her turn is erased, her one-off and his 2 go to the scrap, and he plays a normal turn. On her next turn Alice may counter back: that erases Blake's whole turn, both his counter and his action, the game returns to just after her one-off, both 2s go to the scrap, and she plays a normal turn. Blake may do the same on his next turn, and so on.

## Freeze
- A card returned by a 9 is frozen for the turn that starts right after the 9, and for no other. A frozen 2 can't counter.
- A counter clears every freeze.

## Win timing
- A win on a turn that **can't** be countered happens at once.
- If you reach your target on a **counterable** turn, your turn ends at once. If your opponent holds no unfrozen 2, you win. If they hold one, they get **"Not So Fast!"**: their turn starts early with exactly two choices:
  - **Counter** your winning turn with a 2 (the normal counter). You may counter back at your next turn start, which brings back your winning position and this rule again.
  - **Let it happen**: the game ends and you win.
- If both players reach their targets in one counterable turn, the mover's claim comes first. A counter erases the opponent's points as well, so points from a turn that can still be erased never win unless the player who could erase it lets it happen.

## The 4
- Alice plays a 4. Nothing is discarded yet. At the start of Blake's turn, he either counters or discards two, then takes his turn as usual. Paying is not his action; his turn goes on.
- If Blake counters, the 4 is erased and so is the debt.
- If Alice counters back, the debt comes back: Blake owes 2 again at his next turn start, where he can counter again or pay.
- Two 4s only stack when Alice counters back and then plays a second 4: Blake then owes 4, paid two at a time. A counter there clears both.
- **Paying again**: if Blake pays and Alice then counters his turn, the cards he paid come back to his hand and he owes 2 again; he may pay with different cards.
- **Small hands**: a debt bigger than the hand means discarding the whole hand; an empty hand pays nothing and the debt clears. Holding 2♣ and 7♦, Blake can counter and keep the 7♦.

## Notes / Clarifications
- **Jacks vs. Queens**: a Queen on your side prevents your point cards from being stolen by Jacks.
- **Jack chain-steal**: a Jack may be played onto a point card that already has a Jack on it; control transfers to the new Jack's owner.
- **2 vs. a Jack stack**: a 2 removes only the **top** Jack (to scrap); buried Jacks can't be targeted. Control passes to the owner of the next Jack down, or to the card's original owner if no Jacks remain. The point card moves to the new controller's side and counts for them.
- **6 vs. Jacks**: a 6 scraps every Jack in every stack; each point card returns to its original owner.
- **Glasses 8**: any 8, regardless of suit.
- **Win check**: after your action resolves, and again at the start of your turn. Your opponent's action can hand you points (a 2 popping their own Jack, or a 6), but you win only once your turn begins. On a counterable turn, see *Win timing*.

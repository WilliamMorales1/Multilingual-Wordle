package api

import (
	"slices"
	"testing"

	"wordgo/internal/store"
)

// A losing game has to count as a game played, break the current streak, and
// drag the win percentage below 100. The stats query used to select only
// won games, which made every one of these numbers a lie.
func TestSummarizeGamesCountsLosses(t *testing.T) {
	games := []store.Game{
		{ID: 1, Status: "won"},
		{ID: 2, Status: "won"},
		{ID: 3, Status: "lost"},
		{ID: 4, Status: "won"},
	}

	s := summarizeGames(games)

	if s.Played != 4 {
		t.Errorf("Played = %d, want 4", s.Played)
	}
	if s.Won != 3 {
		t.Errorf("Won = %d, want 3", s.Won)
	}
	if s.WinPct != 75 {
		t.Errorf("WinPct = %d, want 75", s.WinPct)
	}
	if s.MaxStreak != 2 {
		t.Errorf("MaxStreak = %d, want 2", s.MaxStreak)
	}
	if s.CurrentStreak != 1 {
		t.Errorf("CurrentStreak = %d, want 1 (the loss at #3 broke the older run)", s.CurrentStreak)
	}
	if want := []uint{1, 2, 4}; !slices.Equal(s.WonIDs, want) {
		t.Errorf("WonIDs = %v, want %v", s.WonIDs, want)
	}
}

func TestSummarizeGamesEdgeCases(t *testing.T) {
	t.Run("no games", func(t *testing.T) {
		s := summarizeGames(nil)
		if s.Played != 0 || s.Won != 0 || s.WinPct != 0 || s.CurrentStreak != 0 || s.MaxStreak != 0 {
			t.Errorf("empty history = %+v, want all zero", s)
		}
	})

	t.Run("all losses", func(t *testing.T) {
		s := summarizeGames([]store.Game{{ID: 1, Status: "lost"}, {ID: 2, Status: "lost"}})
		if s.Played != 2 {
			t.Errorf("Played = %d, want 2", s.Played)
		}
		if s.WinPct != 0 {
			t.Errorf("WinPct = %d, want 0", s.WinPct)
		}
		if s.WonIDs != nil {
			t.Errorf("WonIDs = %v, want none", s.WonIDs)
		}
	})

	t.Run("loss last ends the current streak", func(t *testing.T) {
		s := summarizeGames([]store.Game{{ID: 1, Status: "won"}, {ID: 2, Status: "lost"}})
		if s.CurrentStreak != 0 {
			t.Errorf("CurrentStreak = %d, want 0", s.CurrentStreak)
		}
		if s.MaxStreak != 1 {
			t.Errorf("MaxStreak = %d, want 1", s.MaxStreak)
		}
	})
}

func TestWinMessageClampsToRange(t *testing.T) {
	if got := winMessage(1); got != "Genius!" {
		t.Errorf("winMessage(1) = %q, want %q", got, "Genius!")
	}
	if got := winMessage(0); got != "Genius!" {
		t.Errorf("winMessage(0) = %q, want %q", got, "Genius!")
	}
	if got := winMessage(99); got != winMessages[len(winMessages)-1] {
		t.Errorf("winMessage(99) = %q, want %q", got, winMessages[len(winMessages)-1])
	}
}

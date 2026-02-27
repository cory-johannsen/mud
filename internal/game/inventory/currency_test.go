package inventory

import (
	"testing"

	"pgregory.net/rapid"
)

func TestCurrency_Decompose_Zero(t *testing.T) {
	crates, clips, rounds := DecomposeRounds(0)
	if crates != 0 || clips != 0 || rounds != 0 {
		t.Fatalf("expected 0,0,0 got %d,%d,%d", crates, clips, rounds)
	}
}

func TestCurrency_Decompose_ExactCrate(t *testing.T) {
	crates, clips, rounds := DecomposeRounds(500)
	if crates != 1 || clips != 0 || rounds != 0 {
		t.Fatalf("expected 1,0,0 got %d,%d,%d", crates, clips, rounds)
	}
}

func TestCurrency_Decompose_Mixed(t *testing.T) {
	crates, clips, rounds := DecomposeRounds(1042)
	if crates != 2 || clips != 1 || rounds != 17 {
		t.Fatalf("expected 2,1,17 got %d,%d,%d", crates, clips, rounds)
	}
}

func TestCurrency_FormatRounds_Zero(t *testing.T) {
	got := FormatRounds(0)
	if got != "0 Rounds" {
		t.Fatalf("expected %q got %q", "0 Rounds", got)
	}
}

func TestCurrency_FormatRounds_OnlyRounds(t *testing.T) {
	got := FormatRounds(17)
	if got != "17 Rounds" {
		t.Fatalf("expected %q got %q", "17 Rounds", got)
	}
}

func TestCurrency_FormatRounds_Mixed(t *testing.T) {
	got := FormatRounds(1042)
	want := "2 Crates, 1 Clip, 17 Rounds"
	if got != want {
		t.Fatalf("expected %q got %q", want, got)
	}
}

func TestCurrency_FormatRounds_NoCrates(t *testing.T) {
	got := FormatRounds(80)
	want := "3 Clips, 5 Rounds"
	if got != want {
		t.Fatalf("expected %q got %q", want, got)
	}
}

func TestCurrency_FormatRounds_SingleRound(t *testing.T) {
	got := FormatRounds(1)
	want := "1 Round"
	if got != want {
		t.Fatalf("expected %q got %q", want, got)
	}
}

func TestProperty_Decompose_Roundtrips(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		total := rapid.IntRange(0, 1_000_000).Draw(t, "total")
		crates, clips, rounds := DecomposeRounds(total)

		if crates*RoundsPerCrate+clips*RoundsPerClip+rounds != total {
			t.Fatalf("roundtrip failed: %d*500+%d*25+%d != %d", crates, clips, rounds, total)
		}
		if clips < 0 || clips >= 20 {
			t.Fatalf("clips out of range: %d", clips)
		}
		if rounds < 0 || rounds >= RoundsPerClip {
			t.Fatalf("rounds out of range: %d", rounds)
		}
	})
}

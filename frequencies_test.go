package hebmorph

import (
	"slices"
	"testing"
)

// TestReadingsOrderedByFrequency is the point of the frequency table: hspell
// offers אביהו as the name "Avihu" first, and a reader almost always means
// "his father". Ranking by Modern Hebrew usage puts that reading first.
func TestReadingsOrderedByFrequency(t *testing.T) {
	a := newAnalyzer(t)
	for _, want := range []struct{ word, first, buried string }{
		{"אביהו", "אב", "שונות"},        // "his father" over the name Avihu
		{"טלפוניה", "טלפון", "טלפוניה"}, // "her telephones" over "telephony"
		{"מידען", "מידע", "מידען"},      // "their information" over "information specialist"
	} {
		readings := a.Analyze(want.word).Splits[0].Readings
		if len(readings) < 2 {
			t.Errorf("%q: want at least 2 readings, got %d", want.word, len(readings))
			continue
		}
		if got := readings[0].Stem; got != want.first {
			t.Errorf("%q: first reading is of %q, want %q", want.word, got, want.first)
		}
		buried := slices.IndexFunc(readings, func(r Reading) bool { return r.Stem == want.buried })
		if buried <= 0 {
			t.Errorf("%q: want the %q reading kept but demoted, got it at index %d", want.word, want.buried, buried)
		}
	}
}

// TestReadingsSortedByRank checks the ordering itself across the whole
// dictionary, rather than trusting the handful of words above.
func TestReadingsSortedByRank(t *testing.T) {
	a := newAnalyzer(t)
	for _, word := range a.dict.words {
		for _, s := range a.Analyze(word).Splits {
			for i := 1; i < len(s.Readings); i++ {
				prev, cur := s.Readings[i-1], s.Readings[i]
				if sortRank(prev.Rank) > sortRank(cur.Rank) {
					t.Fatalf("%q: reading %d (rank %d) sorts before reading %d (rank %d)",
						word, i-1, prev.Rank, i, cur.Rank)
				}
			}
		}
	}
}

// TestFrequencyTiesKeepDictionaryOrder covers the other half of the sort: two
// readings of one lemma share a rank, so hspell's own order has to survive
// between them. מלכה carries מלך twice, as a verb and as a possessed noun.
func TestFrequencyTiesKeepDictionaryOrder(t *testing.T) {
	a := newAnalyzer(t)
	readings := a.Analyze("מלכה").Splits[0].Readings

	verb := slices.IndexFunc(readings, func(r Reading) bool {
		return r.Stem == "מלך" && r.Features.PartOfSpeech == "verb"
	})
	noun := slices.IndexFunc(readings, func(r Reading) bool {
		return r.Stem == "מלך" && r.Features.PartOfSpeech == "noun"
	})
	if verb < 0 || noun < 0 {
		t.Fatalf("want both מלך readings of מלכה, got %+v", readings)
	}
	if readings[verb].Rank != readings[noun].Rank {
		t.Errorf("the two מלך readings rank %d and %d, want one rank per lemma",
			readings[verb].Rank, readings[noun].Rank)
	}
	if verb > noun {
		t.Errorf("tied readings reordered: verb at %d, noun at %d, want the dictionary's order kept", verb, noun)
	}
}

// TestFrequencyCoverage checks that the embedded table still ranks nearly
// every lemma, so a half-finished regeneration cannot ship unnoticed. It
// mirrors TestGlossCoverage.
func TestFrequencyCoverage(t *testing.T) {
	a := newAnalyzer(t)

	lemmas := map[int32]bool{}
	for word, rs := range a.dict.readings {
		for _, r := range rs {
			lemma, _ := a.dict.lemmaOf(int32(word), r.stemIndex)
			lemmas[lemma] = true
		}
	}
	ranked := 0
	for lemma := range lemmas {
		if a.dict.ranks[lemma] > 0 {
			ranked++
		}
	}

	const wantAtLeast = 0.99
	if got := float64(ranked) / float64(len(lemmas)); got < wantAtLeast {
		t.Errorf("frequency coverage = %.1f%% (%d of %d lemmas), want at least %.0f%%",
			100*got, ranked, len(lemmas), 100*wantAtLeast)
	}
}

// TestRanksAreAPermutation checks the blob's central invariant: a rank is a
// position, so no two lemmas may hold the same one and none may be skipped.
func TestRanksAreAPermutation(t *testing.T) {
	a := newAnalyzer(t)

	holder := map[uint16]int32{}
	var highest uint16
	for word, rank := range a.dict.ranks {
		if rank == 0 {
			continue
		}
		if prev, dup := holder[rank]; dup {
			t.Fatalf("rank %d is held by both %q and %q", rank, a.dict.words[prev], a.dict.words[word])
		}
		holder[rank] = int32(word)
		highest = max(highest, rank)
	}
	if int(highest) != len(holder) {
		t.Errorf("%d lemmas ranked but the highest rank is %d, want a gapless 1..%d", len(holder), highest, len(holder))
	}
}

// TestUnrankedSortsLast covers the lookup's empty case and the ordering it
// feeds: a lemma the table misses falls to the back instead of leading on its
// zero rank.
func TestUnrankedSortsLast(t *testing.T) {
	a := newAnalyzer(t)
	if got := a.dict.rankOf(-1, -1); got != 0 {
		t.Errorf("rankOf(-1, -1) = %d, want 0", got)
	}
	if got := sortRank(0); got <= sortRank(65534) {
		t.Errorf("sortRank(0) = %d, want it above every real rank", got)
	}
	if got := sortRank(7); got != 7 {
		t.Errorf("sortRank(7) = %d, want 7", got)
	}
}

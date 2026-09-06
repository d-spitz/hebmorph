package hebmorph

import (
	"slices"
	"testing"
)

// TestGlossesFollowPartOfSpeech is the point of keying glosses by part of
// speech rather than by stem alone: מלכה has two readings of the stem מלך, and
// each must get the sense that belongs to it.
func TestGlossesFollowPartOfSpeech(t *testing.T) {
	a := newAnalyzer(t)
	readings := a.Analyze("מלכה").Splits[0].Readings

	for _, want := range []struct {
		stem, pos, gloss string
	}{
		{"מלך", "verb", "to reign"},
		{"מלכה", "noun", "queen"},
		{"מלך", "noun", "king"},
	} {
		i := slices.IndexFunc(readings, func(r Reading) bool {
			return r.Stem == want.stem && r.Features.PartOfSpeech == want.pos
		})
		if i < 0 {
			t.Errorf("no %s reading of %q", want.pos, want.stem)
			continue
		}
		if got := readings[i].Glosses; !slices.Contains(got, want.gloss) {
			t.Errorf("%s %q: glosses = %q, want them to include %q", want.pos, want.stem, got, want.gloss)
		}
	}
}

// TestGlossCoverage checks that the embedded table still covers nearly every
// stem, so a half-finished regeneration cannot ship unnoticed.
func TestGlossCoverage(t *testing.T) {
	a := newAnalyzer(t)

	stems := map[int32]bool{}
	for _, rs := range a.dict.readings {
		for _, r := range rs {
			stems[r.stemIndex] = true
		}
	}
	translated := 0
	for stem := range stems {
		if len(a.dict.glosses[stem]) > 0 {
			translated++
		}
	}

	const wantAtLeast = 0.99
	if got := float64(translated) / float64(len(stems)); got < wantAtLeast {
		t.Errorf("gloss coverage = %.1f%% (%d of %d stems), want at least %.0f%%",
			100*got, translated, len(stems), 100*wantAtLeast)
	}
}

// TestGlossesUnknownStem covers the lookup's empty cases: an index with no
// entry, and an entry with no group for the reading's part of speech.
func TestGlossesUnknownStem(t *testing.T) {
	a := newAnalyzer(t)
	if got := a.dict.glossesOf(-1, dNoun); got != nil {
		t.Errorf("glossesOf(-1) = %q, want nil", got)
	}
	queen := a.dict.index["מלכה"]
	if got := a.dict.glossesOf(queen, dVerb); got != nil {
		t.Errorf("glossesOf(מלכה, verb) = %q, want nil — it is only a noun", got)
	}
}

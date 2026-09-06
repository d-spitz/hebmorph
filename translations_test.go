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

// TestGlossesFollowLemma covers hspell's catch-all words, which are keyed by
// themselves rather than by a stem. אדם carries both kinds at once, in the
// same part of speech, so it is the case that proves the flag works.
func TestGlossesFollowLemma(t *testing.T) {
	a := newAnalyzer(t)
	for _, want := range []struct {
		word, gloss string
		proper      bool
	}{
		{"אדם", "man", false},   // through its stem
		{"אדם", "Adam", true},   // through the catch-all
		{"צה\"ל", "IDF", true},  // only the catch-all
		{"עכשיו", "now", false}, // a particle, no part of speech at all
	} {
		var got []string
		for _, r := range a.Analyze(want.word).Splits[0].Readings {
			if r.Features.ProperNoun == want.proper {
				got = r.Glosses
				break
			}
		}
		if !slices.Contains(got, want.gloss) {
			t.Errorf("%q (proper_noun=%v): glosses = %q, want them to include %q",
				want.word, want.proper, got, want.gloss)
		}
	}
}

// TestGlossCoverage checks that the embedded table still covers nearly every
// lemma, so a half-finished regeneration cannot ship unnoticed.
func TestGlossCoverage(t *testing.T) {
	a := newAnalyzer(t)

	lemmas := map[int32]bool{}
	for word, rs := range a.dict.readings {
		for _, r := range rs {
			lemma := r.stemIndex
			if lemma == a.dict.miscStem {
				lemma = int32(word)
			}
			lemmas[lemma] = true
		}
	}
	translated := 0
	for lemma := range lemmas {
		if len(a.dict.glosses[lemma]) > 0 {
			translated++
		}
	}

	const wantAtLeast = 0.99
	if got := float64(translated) / float64(len(lemmas)); got < wantAtLeast {
		t.Errorf("gloss coverage = %.1f%% (%d of %d lemmas), want at least %.0f%%",
			100*got, translated, len(lemmas), 100*wantAtLeast)
	}
}

// TestGlossesUnknownLemma covers the lookup's empty cases: an index with no
// entry, and an entry with no group for the reading's part of speech.
func TestGlossesUnknownLemma(t *testing.T) {
	a := newAnalyzer(t)
	if got := a.dict.glossesOf(-1, -1, dNoun); got != nil {
		t.Errorf("glossesOf(-1) = %q, want nil", got)
	}
	queen := a.dict.index["מלכה"]
	if got := a.dict.glossesOf(queen, queen, dVerb); got != nil {
		t.Errorf("glossesOf(מלכה, verb) = %q, want nil — it is only a noun", got)
	}
}

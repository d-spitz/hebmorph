package hebmorph

import (
	"slices"
	"testing"
)

// findReading returns the first reading matching want, or a zero Reading.
func findReading(rs []Reading, match func(Reading) bool) (Reading, bool) {
	if i := slices.IndexFunc(rs, match); i >= 0 {
		return rs[i], true
	}
	return Reading{}, false
}

// TestLemmasSplitHomographs is the point of keying meaning by lemma rather
// than by stem. Hebrew writes no vowels, so three unrelated words are all
// spelled את, and hspell files two of them under one stem. Each reading has to
// come back with its own sense.
func TestLemmasSplitHomographs(t *testing.T) {
	a := newAnalyzer(t)
	for _, want := range []struct {
		word, pos, gloss string
	}{
		{"את", "particle", "direct object marker"},   // et
		{"את", "pronoun", "you (feminine singular)"}, // at
		{"את", "noun", "plowshare"},                  // the tool
		{"אתם", "preposition", "with"},               // ittam
		{"אתם", "pronoun", "you (masculine plural)"}, // atem
		{"אותו", "particle", "direct object marker"}, // oto
		{"אתי", "preposition", "with"},               // itti
	} {
		readings := a.Analyze(want.word).Splits[0].Readings
		r, ok := findReading(readings, func(r Reading) bool {
			return r.Features.PartOfSpeech == want.pos && slices.Contains(r.Glosses, want.gloss)
		})
		if !ok {
			t.Errorf("%q: no %s reading glossed %q; got %+v", want.word, want.pos, want.gloss, readings)
			continue
		}
		if r.Lemma == "" {
			t.Errorf("%q (%s): reading has no lemma", want.word, want.pos)
		}
	}
}

// TestGlossesFollowPartOfSpeech is why one spelling can hold several lemmas
// that differ only in part of speech: מלכה has two readings of the stem מלך,
// and each must get the sense that belongs to it.
func TestGlossesFollowPartOfSpeech(t *testing.T) {
	a := newAnalyzer(t)
	readings := a.Analyze("מלכה").Splits[0].Readings

	for _, want := range []struct{ stem, pos, gloss string }{
		{"מלך", "verb", "to reign"},
		{"מלכה", "noun", "queen"},
		{"מלך", "noun", "king"},
	} {
		r, ok := findReading(readings, func(r Reading) bool {
			return r.Stem == want.stem && r.Features.PartOfSpeech == want.pos
		})
		if !ok {
			t.Errorf("no %s reading of %q", want.pos, want.stem)
			continue
		}
		if !slices.Contains(r.Glosses, want.gloss) {
			t.Errorf("%s %q: glosses = %q, want them to include %q", want.pos, want.stem, r.Glosses, want.gloss)
		}
	}
}

// TestLemmaDiffersFromStem covers the words hspell gives no stem: they all
// name its שונות catch-all, so the lemma is the word itself. אדם carries both
// kinds at once, in one part of speech, which is what makes it the case worth
// testing.
func TestLemmaDiffersFromStem(t *testing.T) {
	a := newAnalyzer(t)
	readings := a.Analyze("אדם").Splits[0].Readings

	common, ok := findReading(readings, func(r Reading) bool { return slices.Contains(r.Glosses, "man") })
	if !ok {
		t.Fatalf("no common-noun reading of אדם: %+v", readings)
	}
	if common.Lemma != "אדם" || common.Stem != "אדם" {
		t.Errorf("common noun: lemma %q stem %q, want both אדם", common.Lemma, common.Stem)
	}

	name, ok := findReading(readings, func(r Reading) bool { return slices.Contains(r.Glosses, "Adam") })
	if !ok {
		t.Fatalf("no proper-noun reading of אדם: %+v", readings)
	}
	if name.Stem != "שונות" {
		t.Errorf("proper noun: stem %q, want hspell's catch-all שונות", name.Stem)
	}
	if name.Lemma != "אדם" {
		t.Errorf("proper noun: lemma %q, want אדם — a word with no stem is its own lemma", name.Lemma)
	}
}

// TestAddedSpellings covers internal/gen/additions.json: hspell rejects the
// ktiv male spelling of "with" on prescriptive grounds (spellinghints says to
// write אתו, not איתו), but it is what Modern Hebrew uses.
func TestAddedSpellings(t *testing.T) {
	a := newAnalyzer(t)
	for _, word := range []string{
		"איתי", "איתך", "איתו", "איתה", "איתנו", "איתכם", "איתכן", "איתם", "איתן",
		"עימי", "עימך", "עימו", "עימה", "עימנו", "עימכם", "עימכן", "עימם", "עימן",
	} {
		an := a.Analyze(word)
		if !an.Exists {
			t.Errorf("%q should be a word", word)
			continue
		}
		r, ok := findReading(an.Splits[0].Readings, func(r Reading) bool {
			return slices.Contains(r.Glosses, "with")
		})
		if !ok {
			t.Errorf("%q: no reading glossed \"with\"; got %+v", word, an.Splits[0].Readings)
			continue
		}
		if r.Features.PartOfSpeech != "preposition" {
			t.Errorf("%q: part of speech %q, want preposition", word, r.Features.PartOfSpeech)
		}
	}
}

// TestRefinedPartOfSpeech checks that lemmas carry a part of speech for the
// words hspell records none for. hspell only ever says noun, verb or
// adjective, so every particle and pronoun in the language came back blank.
func TestRefinedPartOfSpeech(t *testing.T) {
	a := newAnalyzer(t)
	for _, want := range []struct{ word, pos string }{
		{"של", "preposition"},
		{"בגלל", "preposition"},
		{"לבד", "adverb"},
		{"יש", "particle"},
		{"את", "pronoun"},
	} {
		readings := a.Analyze(want.word).Splits[0].Readings
		if _, ok := findReading(readings, func(r Reading) bool {
			return r.Features.PartOfSpeech == want.pos
		}); !ok {
			t.Errorf("%q: no %s reading; got %+v", want.word, want.pos, readings)
		}
	}
}

// TestReadingsOrderedByFrequency is the point of ranking: hspell offers אביהו
// as the name "Avihu" first, and a reader almost always means "his father".
func TestReadingsOrderedByFrequency(t *testing.T) {
	a := newAnalyzer(t)
	for _, want := range []struct{ word, first, buried string }{
		{"אביהו", "אב", "אביהו"},        // "his father" over the name Avihu
		{"טלפוניה", "טלפון", "טלפוניה"}, // "her telephones" over "telephony"
		{"מידען", "מידע", "מידען"},      // "their information" over "information specialist"
	} {
		readings := a.Analyze(want.word).Splits[0].Readings
		if len(readings) < 2 {
			t.Errorf("%q: want at least 2 readings, got %d", want.word, len(readings))
			continue
		}
		if got := readings[0].Lemma; got != want.first {
			t.Errorf("%q: first reading is of %q, want %q", want.word, got, want.first)
		}
		buried := slices.IndexFunc(readings, func(r Reading) bool { return r.Lemma == want.buried })
		if buried <= 0 {
			t.Errorf("%q: want the %q reading kept but demoted, got it at index %d", want.word, want.buried, buried)
		}
	}
}

// TestReadingsSortedByRank checks the ordering across the whole dictionary,
// rather than trusting the handful of words above.
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

// TestFrequencyTiesKeepDictionaryOrder covers the other half of the sort: rank
// is counted per spelling, so lemmas sharing one tie and hspell's own order
// has to survive between them. מלכה carries מלך twice, as a verb and a noun.
func TestFrequencyTiesKeepDictionaryOrder(t *testing.T) {
	a := newAnalyzer(t)
	readings := a.Analyze("מלכה").Splits[0].Readings

	verb := slices.IndexFunc(readings, func(r Reading) bool {
		return r.Lemma == "מלך" && r.Features.PartOfSpeech == "verb"
	})
	noun := slices.IndexFunc(readings, func(r Reading) bool {
		return r.Lemma == "מלך" && r.Features.PartOfSpeech == "noun"
	})
	if verb < 0 || noun < 0 {
		t.Fatalf("want both מלך readings of מלכה, got %+v", readings)
	}
	if readings[verb].Rank != readings[noun].Rank {
		t.Errorf("the two מלך readings rank %d and %d, want one rank per spelling",
			readings[verb].Rank, readings[noun].Rank)
	}
	if verb > noun {
		t.Errorf("tied readings reordered: verb at %d, noun at %d, want the dictionary's order kept", verb, noun)
	}
}

// TestLemmaCoverage checks that the embedded table still resolves and glosses
// nearly every reading, so a half-finished regeneration cannot ship unnoticed.
func TestLemmaCoverage(t *testing.T) {
	a := newAnalyzer(t)

	total, resolved, glossed, ranked := 0, 0, 0, 0
	for word, rs := range a.dict.readings {
		for i := range rs {
			total++
			l := a.dict.lemmaAt(int32(word), i)
			if l == nil {
				continue
			}
			resolved++
			if len(l.English) > 0 {
				glossed++
			}
			if l.Rank > 0 {
				ranked++
			}
		}
	}
	if resolved != total {
		t.Errorf("%d of %d readings have no lemma, want every one resolved", total-resolved, total)
	}
	const wantAtLeast = 0.99
	for _, c := range []struct {
		name string
		got  int
	}{{"glossed", glossed}, {"ranked", ranked}} {
		if f := float64(c.got) / float64(total); f < wantAtLeast {
			t.Errorf("%s readings = %.1f%% (%d of %d), want at least %.0f%%",
				c.name, 100*f, c.got, total, 100*wantAtLeast)
		}
	}
}

// TestRanksAreAPermutation checks the table's central invariant: a rank is a
// position among spellings, so two lemmas share one only by sharing a
// spelling, and no rank is skipped.
func TestRanksAreAPermutation(t *testing.T) {
	a := newAnalyzer(t)

	holder := map[uint16]int32{}
	var highest uint16
	for _, l := range a.dict.lemmas {
		if l.Rank == 0 {
			continue
		}
		if prev, dup := holder[l.Rank]; dup && prev != l.Word {
			t.Fatalf("rank %d is held by both %q and %q", l.Rank, a.dict.words[prev], a.dict.words[l.Word])
		}
		holder[l.Rank] = l.Word
		highest = max(highest, l.Rank)
	}
	if int(highest) != len(holder) {
		t.Errorf("%d spellings ranked but the highest rank is %d, want a gapless 1..%d",
			len(holder), highest, len(holder))
	}
}

// TestPartOfSpeechIsKnown guards the vocabulary: a lemma may only report a
// part of speech the API documents.
func TestPartOfSpeechIsKnown(t *testing.T) {
	a := newAnalyzer(t)
	known := map[string]bool{
		"other": true, "noun": true, "verb": true, "adjective": true,
		"particle": true, "preposition": true, "pronoun": true,
		"conjunction": true, "adverb": true, "interjection": true,
	}
	for _, l := range a.dict.lemmas {
		if !known[l.POS] {
			t.Fatalf("lemma %q reports unknown part of speech %q", a.dict.words[l.Word], l.POS)
		}
	}
}

// TestEveryLemmaIsTyped guards the audits that typed hspell's function words.
// hspell records a part of speech for nouns, verbs and adjectives only, so
// every particle, pronoun and preposition in the language would otherwise come
// back blank; the lemma table now names one for all of them.
func TestEveryLemmaIsTyped(t *testing.T) {
	a := newAnalyzer(t)
	var untyped []string
	for _, l := range a.dict.lemmas {
		if l.POS == "" || l.POS == "other" {
			untyped = append(untyped, a.dict.words[l.Word])
		}
	}
	if len(untyped) > 0 {
		if len(untyped) > 10 {
			untyped = untyped[:10]
		}
		t.Errorf("%d lemmas have no part of speech, e.g. %q", len(untyped), untyped)
	}
}

// TestUnrankedSortsLast checks that a lemma the corpus never saw falls to the
// back instead of leading on its zero rank.
func TestUnrankedSortsLast(t *testing.T) {
	if got := sortRank(0); got <= sortRank(65534) {
		t.Errorf("sortRank(0) = %d, want it above every real rank", got)
	}
	if got := sortRank(7); got != 7 {
		t.Errorf("sortRank(7) = %d, want 7", got)
	}
}

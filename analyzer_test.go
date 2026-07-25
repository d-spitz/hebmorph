package hebmorph

import (
	"sync"
	"testing"
)

func newAnalyzer(t testing.TB) *Analyzer {
	t.Helper()
	a, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return a
}

func TestWholeWordAndSplits(t *testing.T) {
	a := newAnalyzer(t)
	got := a.Analyze("מלכה")
	if !got.Exists {
		t.Fatal("מלכה should exist")
	}
	if len(got.Splits) != 2 {
		t.Fatalf("want 2 splits, got %d", len(got.Splits))
	}
	whole := got.Splits[0]
	if !whole.WholeWord || whole.Prefix != "" || whole.Base != "מלכה" {
		t.Errorf("unexpected first split: %+v", whole)
	}
	r0 := whole.Readings[0]
	if r0.Stem != "מלך" || r0.Features.PartOfSpeech != "verb" ||
		r0.Features.Tense != "past" || r0.Features.Person != "3" ||
		r0.Features.Number != "singular" || r0.Features.Gender != "feminine" {
		t.Errorf("unexpected reading: %+v", r0)
	}
	// possessive suffix
	poss := whole.Readings[2].Features.Possessive
	if poss == nil || poss.Gender != "feminine" || poss.Person != "3" || poss.Number != "singular" {
		t.Errorf("unexpected possessive: %+v", poss)
	}
}

func TestPrefixSplit(t *testing.T) {
	a := newAnalyzer(t)
	got := a.Analyze("להחזיר")
	if len(got.Splits) != 1 {
		t.Fatalf("want 1 split, got %d: %+v", len(got.Splits), got.Splits)
	}
	s := got.Splits[0]
	if s.Prefix != "ל" || s.Base != "החזיר" || s.WholeWord {
		t.Errorf("unexpected split: %+v", s)
	}
	if s.Readings[0].Features.Tense != "infinitive" {
		t.Errorf("want infinitive, got %+v", s.Readings[0].Features)
	}
}

func TestOutOfVocabulary(t *testing.T) {
	a := newAnalyzer(t)
	got := a.Analyze("אבדך")
	if got.Exists {
		t.Error("אבדך should not exist")
	}
	if len(got.Splits) != 0 {
		t.Errorf("want no splits, got %+v", got.Splits)
	}
}

func TestGimatria(t *testing.T) {
	a := newAnalyzer(t)
	got := a.Analyze("א'")
	if got.Gimatria == nil || *got.Gimatria != 1 {
		t.Errorf("want gimatria 1, got %v", got.Gimatria)
	}
}

func TestConcurrentAnalyze(t *testing.T) {
	a := newAnalyzer(t)
	words := []string{"מלכה", "שלום", "להחזיר", "אבד", "חזיר"}
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(w string) {
			defer wg.Done()
			if !a.Analyze(w).Exists {
				t.Errorf("%q should exist", w)
			}
		}(words[i%len(words)])
	}
	wg.Wait()
}

func BenchmarkNew(b *testing.B) {
	for b.Loop() {
		if _, err := New(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAnalyze(b *testing.B) {
	a := newAnalyzer(b)

	for b.Loop() {
		a.Analyze("מלכה")
	}
}

package hebmorph

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
)

// dictionary.json.gz is hspell's word list and morphology in UTF-8, generated
// by ./internal/gen, plus the handful of spellings in
// internal/gen/additions.json that hspell rejects but Modern Hebrew uses. It
// is gzipped JSON: a hand-packed binary encoding saved only 10% after gzip,
// which did not pay for a codec to keep in sync on both sides of the build.
//
//go:embed data/dictionary.json.gz
var dictGz []byte

// reading is one morphological interpretation, referencing its stem by index
// into the word table.
type reading struct {
	stemIndex int32
	dmask     int32
}

// dictionary holds the whole word list in memory, keyed for O(1) lookup.
type dictionary struct {
	words    []string         // sorted; stems reference words by index
	index    map[string]int32 // word -> position in words
	specs    []byte           // prefix specifier per word
	readings [][]reading
	lemmas   []lemma   // the sense table; see lemmas.go
	lemmaOf  [][]int32 // parallel to readings: the lemma each one means
}

// dictFile mirrors blob.Dict, the on-disk shape.
type dictFile struct {
	Words    []string `json:"words"`
	Specs    []int    `json:"specs"`
	Readings [][]struct {
		StemIndex int32 `json:"s"`
		Dmask     int32 `json:"d"`
	} `json:"readings"`
}

func loadDictionary() (*dictionary, error) {
	var f dictFile
	if err := loadGzipJSON(dictGz, &f, "dictionary"); err != nil {
		return nil, err
	}
	n := len(f.Words)
	if len(f.Specs) != n || len(f.Readings) != n {
		return nil, fmt.Errorf("dictionary: %d words but %d specs and %d reading lists",
			n, len(f.Specs), len(f.Readings))
	}

	d := &dictionary{
		words:    f.Words,
		index:    make(map[string]int32, n),
		specs:    make([]byte, n),
		readings: make([][]reading, n),
	}
	for i := 0; i < n; i++ {
		d.index[f.Words[i]] = int32(i)
		d.specs[i] = byte(f.Specs[i])
		rs := make([]reading, len(f.Readings[i]))
		for j, r := range f.Readings[i] {
			if r.StemIndex < 0 || int(r.StemIndex) >= n {
				return nil, fmt.Errorf("dictionary: word %q reading %d names word %d, which is out of range",
					f.Words[i], j, r.StemIndex)
			}
			rs[j] = reading{stemIndex: r.StemIndex, dmask: r.Dmask}
		}
		d.readings[i] = rs
	}

	if err := d.loadLemmas(); err != nil {
		return nil, err
	}
	return d, nil
}

// specifier returns the word's prefix specifier and whether the word is known.
func (d *dictionary) specifier(word string) (byte, bool) {
	if i, ok := d.index[word]; ok {
		return d.specs[i], true
	}
	return 0, false
}

// loadGzipJSON decodes one of the embedded gzipped JSON blobs into v.
func loadGzipJSON(gz []byte, v any, name string) error {
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return fmt.Errorf("open embedded %s: %w", name, err)
	}
	if err := json.NewDecoder(zr).Decode(v); err != nil {
		return fmt.Errorf("decode %s: %w", name, err)
	}
	return nil
}

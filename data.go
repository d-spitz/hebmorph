package hebmorph

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/binary"
	"fmt"
	"io"
)

// hebrew.dict.gz is the UTF-8 dictionary, generated from hspell's original
// ISO-8859-8 data by ./internal/gen. Format (gzipped): a uvarint word count,
// then for each word (in sorted order): uvarint length + UTF-8 bytes, one
// prefix-specifier byte, uvarint reading count, then that many
// (uvarint stemIndex, uvarint dmask) pairs.
//
//go:embed data/hebrew.dict.gz
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
}

func loadDictionary() (*dictionary, error) {
	zr, err := gzip.NewReader(bytes.NewReader(dictGz))
	if err != nil {
		return nil, fmt.Errorf("open embedded dictionary: %w", err)
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("decompress dictionary: %w", err)
	}

	r := bytes.NewReader(raw)
	n64, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, fmt.Errorf("read word count: %w", err)
	}
	n := int(n64)

	d := &dictionary{
		words:    make([]string, n),
		index:    make(map[string]int32, n),
		specs:    make([]byte, n),
		readings: make([][]reading, n),
	}
	for i := 0; i < n; i++ {
		wlen, err := binary.ReadUvarint(r)
		if err != nil {
			return nil, fmt.Errorf("word %d length: %w", i, err)
		}
		w := make([]byte, wlen)
		if _, err := io.ReadFull(r, w); err != nil {
			return nil, fmt.Errorf("word %d bytes: %w", i, err)
		}
		spec, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("word %d spec: %w", i, err)
		}
		nr, err := binary.ReadUvarint(r)
		if err != nil {
			return nil, fmt.Errorf("word %d reading count: %w", i, err)
		}
		rs := make([]reading, nr)
		for j := range rs {
			stemIdx, err := binary.ReadUvarint(r)
			if err != nil {
				return nil, fmt.Errorf("word %d reading %d stem: %w", i, j, err)
			}
			dmask, err := binary.ReadUvarint(r)
			if err != nil {
				return nil, fmt.Errorf("word %d reading %d dmask: %w", i, j, err)
			}
			rs[j] = reading{stemIndex: int32(stemIdx), dmask: int32(dmask)}
		}

		d.words[i] = string(w)
		d.specs[i] = spec
		d.readings[i] = rs
		d.index[string(w)] = int32(i)
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

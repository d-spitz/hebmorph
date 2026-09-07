// Package hebmorph is a Hebrew morphological analyzer: given a word it returns
// the ways the word can be split into a known prefix and base, and for each
// base the possible morphological readings (part of speech, gender, number,
// tense, ...).
//
// It is a modern, UTF-8-native rewrite of the analysis core of hspell
// (http://hspell.ivrix.org.il). The dictionary is embedded in the binary, so
// an Analyzer is self-contained:
//
//	a, err := hebmorph.New()
//	if err != nil { ... }
//	analysis := a.Analyze("מלכה")
package hebmorph

import (
	"cmp"
	"math"
	"slices"
)

// Analysis is the result of analyzing one word.
type Analysis struct {
	Word   string `json:"word"`
	Exists bool   `json:"exists"`
	// Gimatria is set when the word is a canonical Hebrew numeral (e.g. י"ד,
	// 14), giving its numeric value. See the TODO on gimatriaString: numerals
	// using a multi-rune digit are currently not recognized.
	Gimatria *int    `json:"gimatria,omitempty"`
	Splits   []Split `json:"splits"`
}

// Split is one way to divide the word into a prefix and a base word.
type Split struct {
	Prefix    string    `json:"prefix"`
	Base      string    `json:"base"`
	WholeWord bool      `json:"whole_word"` // true when there is no prefix
	Readings  []Reading `json:"readings"`
}

// Reading is one morphological interpretation of a base word.
type Reading struct {
	Stem     string   `json:"stem"`
	Desc     string   `json:"desc"` // native hspell notation, e.g. "פ,נ,3,יחיד,עבר"
	Features Features `json:"features"`
	// Glosses are the stem's English translations in this reading's part of
	// speech, most representative sense first. It is empty when the stem has
	// no translation for that part of speech. The slice is shared between
	// analyses and must not be modified.
	Glosses []string `json:"glosses,omitempty"`
	// Rank is how common this reading's lemma is in Modern Hebrew: rank 1 is
	// the most frequent lemma in the dictionary, and the least frequent ranks
	// last. Readings are returned in this order. It is 0 for a lemma the
	// frequency table does not cover, which sorts last.
	//
	// It ranks the lemma, not this reading of this word: מלכה is "queen" more
	// often than it is "her king", but מלך is the commoner lemma and so leads.
	// It is a suggested ordering, not a claim about this occurrence.
	Rank uint16 `json:"rank,omitempty"`
}

// prefixEntry is a legal prefix and the specifier mask it supplies. The
// prefixTable is generated in prefixes_data.go.
type prefixEntry struct {
	prefix string
	mask   int
}

// Analyzer holds the loaded dictionary and prefix tree. It is safe for
// concurrent use: Analyze only reads.
type Analyzer struct {
	dict *dictionary
	root *prefixNode
}

// New loads the embedded dictionary and returns a ready-to-use Analyzer.
func New() (*Analyzer, error) {
	d, err := loadDictionary()
	if err != nil {
		return nil, err
	}
	return &Analyzer{dict: d, root: buildPrefixTree()}, nil
}

// prefixNode is a node in the trie of legal prefixes. mask is the specifier a
// word must accept for this prefix to apply.
type prefixNode struct {
	mask     int
	children map[rune]*prefixNode
}

func buildPrefixTree() *prefixNode {
	root := &prefixNode{children: map[rune]*prefixNode{}}
	for _, e := range prefixTable {
		n := root
		for _, r := range e.prefix {
			next := n.children[r]
			if next == nil {
				next = &prefixNode{children: map[rune]*prefixNode{}}
				n.children[r] = next
			}
			n = next
		}
		n.mask = e.mask
	}
	return root
}

// Analyze returns the morphological analysis of a single UTF-8 word.
func (a *Analyzer) Analyze(word string) *Analysis {
	res := &Analysis{Word: word, Splits: []Split{}}
	runes := []rune(word)
	if len(runes) == 0 {
		return res
	}

	// Strip a leading quote/apostrophe and a trailing quote, matching hspell's
	// handling of acronyms and quoted words.
	start := 0
	if runes[0] == '"' || runes[0] == '\'' {
		start = 1
	}
	end := len(runes)
	if end > start && runes[end-1] == '"' {
		end--
	}
	w := runes[start:end]

	ok := a.valid(w)
	if !ok {
		if g := canonicGimatria(w); g != 0 {
			res.Gimatria = &g
			ok = true
		}
	}
	if !ok && len(w) > 0 && w[len(w)-1] == '\'' {
		// retry once without a trailing apostrophe (abbreviation)
		w = w[:len(w)-1]
		ok = a.valid(w)
	}
	if !ok {
		return res
	}

	res.Exists = true
	a.enumSplits(w, func(prefixLen, baseStart, prefixMask int) {
		res.Splits = append(res.Splits, Split{
			Prefix:    string(w[:prefixLen]),
			Base:      string(w[baseStart:]),
			WholeWord: prefixLen == 0,
			Readings:  a.readingsOf(w[baseStart:], prefixMask),
		})
	})
	return res
}

// readingsOf returns the readings of base that are compatible with prefixMask,
// most common in Modern Hebrew first.
func (a *Analyzer) readingsOf(base []rune, prefixMask int) []Reading {
	out := []Reading{}
	i, ok := a.dict.index[string(base)]
	if !ok {
		return out
	}
	for _, rd := range a.dict.readings[i] {
		dmask := int(rd.dmask)
		if prefixSpecifier(dmask)&prefixMask == 0 {
			continue
		}
		out = append(out, Reading{
			Stem:     a.dict.words[rd.stemIndex],
			Desc:     hebrewDesc(dmask),
			Features: decodeFeatures(dmask),
			Glosses:  a.dict.glossesOf(i, rd.stemIndex, dmask),
			Rank:     a.dict.rankOf(i, rd.stemIndex),
		})
	}

	// hspell orders readings by its own criteria, which say nothing about
	// usage: it offers אביהו as the name "Avihu" before "his father". Sorting
	// by rank puts the reading a reader most likely met first. The sort is
	// stable because rank is per lemma: readings that share one (מלך the king
	// and מלך to reign) tie, and keep hspell's order between them.
	slices.SortStableFunc(out, func(a, b Reading) int {
		return cmp.Compare(sortRank(a.Rank), sortRank(b.Rank))
	})
	return out
}

// sortRank orders an unranked reading last rather than first, where its zero
// rank would otherwise put it.
func sortRank(rank uint16) uint16 {
	if rank == 0 {
		return math.MaxUint16
	}
	return rank
}

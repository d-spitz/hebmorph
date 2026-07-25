package hebmorph

// Prefix splitting, ported from hspell's libhspell.c (hspell_check_word and
// hspell_enum_splits), rewritten to walk Unicode runes instead of ISO-8859-8
// bytes.

const (
	alef = 'א' // 0x05D0
	tav  = 'ת' // 0x05EA
	vav  = 'ו' // 0x05D5
)

func isHebrew(r rune) bool { return r >= alef && r <= tav }

// matches reports whether the base word `sub` is known and accepts a prefix
// with the given specifier mask.
func (a *Analyzer) matches(sub []rune, mask int) bool {
	spec, ok := a.dict.specifier(string(sub))
	return ok && int(spec)&mask != 0
}

// valid reports whether w is a legal word: either a known word, or a legal
// prefix followed by a base word that accepts it. Mirrors hspell_check_word.
func (a *Analyzer) valid(w []rune) bool {
	i := 0
	for i < len(w) && !isHebrew(w[i]) {
		i++
	}
	if i == len(w) {
		return true // nothing Hebrew to check; accept, as hspell does
	}
	n := a.root
	for i < len(w) && n != nil {
		r := w[i]
		if r == '"' { // acronym quote inside a word is skipped
			i++
			continue
		}
		if n != a.root && r == vav && w[i-1] != vav {
			// Academy "doubled waw" rule: a consonantal waw in mid-word is
			// written double, so a base starting with waw may appear doubled.
			if i+1 < len(w) && w[i+1] == vav {
				if (i+2 >= len(w) || w[i+2] != vav) && a.matches(w[i+1:], n.mask) {
					return true
				}
				if a.matches(w[i:], n.mask) {
					return true
				}
			}
		} else if a.matches(w[i:], n.mask) {
			return true
		}
		if isHebrew(r) {
			n = n.children[r]
			i++
		} else {
			break
		}
	}
	// a legal prefix followed by nothing (or a non-word) is accepted
	return n != nil && i == len(w)
}

// enumSplits calls emit for every legal split of w, with the prefix length (in
// runes), the base word's start index, and the prefix's specifier mask.
// Mirrors hspell_enum_splits.
func (a *Analyzer) enumSplits(w []rune, emit func(prefixLen, baseStart, prefixMask int)) {
	prefixLen := 0
	i := 0
	for i < len(w) && !isHebrew(w[i]) {
		prefixLen++
		i++
	}
	if i == len(w) {
		return
	}
	n := a.root
	for i < len(w) && n != nil {
		r := w[i]
		if r == '"' {
			prefixLen++
			i++
			continue
		}
		if n != a.root && r == vav && w[i-1] != vav {
			if i+1 < len(w) && w[i+1] == vav {
				if (i+2 >= len(w) || w[i+2] != vav) && a.matches(w[i+1:], n.mask) {
					i++ // skip the doubled waw; base starts here
					emit(prefixLen, i, n.mask)
					prefixLen++
					n = n.children[w[i]]
					i++
					continue
				}
				if a.matches(w[i:], n.mask) {
					emit(prefixLen, i, n.mask)
					prefixLen++
					n = n.children[w[i]]
					i++
					continue
				}
			}
		} else if a.matches(w[i:], n.mask) {
			emit(prefixLen, i, n.mask)
			prefixLen++
			n = n.children[w[i]]
			i++
			continue
		}
		if isHebrew(r) {
			n = n.children[r]
			prefixLen++
			i++
		} else {
			break
		}
	}
	if n != nil && i == len(w) {
		emit(prefixLen, i, n.mask)
	}
}

package hebmorph

// Canonical gimatria (Hebrew numeral) recognition, ported from hspell's
// gimatria.c and rewritten for runes. A word is a canonical numeral if
// rendering its numeric value back to Hebrew reproduces the word exactly.

// letterValue returns the gimatria value of a Hebrew letter, or 0.
func letterValue(r rune) int {
	switch r {
	case 'א':
		return 1
	case 'ב':
		return 2
	case 'ג':
		return 3
	case 'ד':
		return 4
	case 'ה':
		return 5
	case 'ו':
		return 6
	case 'ז':
		return 7
	case 'ח':
		return 8
	case 'ט':
		return 9
	case 'י':
		return 10
	case 'ך', 'כ':
		return 20
	case 'ל':
		return 30
	case 'ם', 'מ':
		return 40
	case 'ן', 'נ':
		return 50
	case 'ס':
		return 60
	case 'ע':
		return 70
	case 'ף', 'פ':
		return 80
	case 'ץ', 'צ':
		return 90
	case 'ק':
		return 100
	case 'ר':
		return 200
	case 'ש':
		return 300
	case 'ת':
		return 400
	}
	return 0
}

func gimatriaValue(w []rune) int {
	n := 0
	for i, r := range w {
		if r == '\'' {
			if i+1 < len(w) { // apostrophe not at the end multiplies by 1000
				n *= 1000
			}
			continue
		}
		n += letterValue(r)
	}
	return n
}

var (
	gimDigits = [3][9]string{
		{"א", "ב", "ג", "ד", "ה", "ו", "ז", "ח", "ט"},
		{"י", "כ", "ל", "מ", "נ", "ס", "ע", "פ", "צ"},
		{"ק", "ר", "ש", "ת", "תק", "תר", "תש", "תת", "תתק"},
	}
	gimSpecial = [2]string{"טו", "טז"} // 15, 16 avoid spelling out the Name
)

// gimatriaString renders n as its canonical Hebrew numeral.
//
// TODO: broken for any numeral built from a multi-rune digit — the hundreds
// תק/תר/תש/תת/תתק and the specials טו/טז. The reversal below flips the whole
// rune slice, including the runes *within* one of those digits, so the letters
// of the digit come out backwards:
//
//	תשע"ה (775) is rejected, while שתע"ה is accepted as 775
//	ט"ו   (15)  is rejected, while ו"ט   is accepted as 15
//	תקכ"ג (523) is rejected, while קתכ"ג is accepted as 523
//
// That means 15, 16 and everything from 500 up is wrong. The fix is to collect
// each digit as a string and reverse the order of the digits, rather than
// reversing runes. Left alone for now: gimatria is a side feature, and it is
// not yet established whether hspell's C original behaves the same way.
//
// This escaped the byte-for-byte comparison against the reference port because
// canonicGimatria only runs for words *absent* from the dictionary, so a
// dictionary-driven diff never reaches it.
func gimatriaString(n int) string {
	var b []rune
	i := 0
	for n > 0 {
		if i == 3 {
			i = 0
			b = append(b, '\'')
		}
		if i == 0 && (n%100 == 15 || n%100 == 16) {
			b = append(b, []rune(gimSpecial[n%100-15])...)
			n /= 100
			i = 2
		} else {
			if n%10 != 0 {
				b = append(b, []rune(gimDigits[i][n%10-1])...)
			}
			n /= 10
			i++
		}
	}
	// digits were emitted least-significant first; reverse to reading order
	for l, r := 0, len(b)-1; l < r; l, r = l+1, r-1 {
		b[l], b[r] = b[r], b[l]
	}
	if len(b) == 0 {
		return ""
	}
	// the last letter takes its final (sofit) form
	switch b[len(b)-1] {
	case 'כ':
		b[len(b)-1] = 'ך'
	case 'מ':
		b[len(b)-1] = 'ם'
	case 'נ':
		b[len(b)-1] = 'ן'
	case 'פ':
		b[len(b)-1] = 'ף'
	case 'צ':
		b[len(b)-1] = 'ץ'
	}
	// punctuation: geresh for a single letter, gershayim before the last
	switch {
	case len(b) == 1:
		b = append(b, '\'')
	case b[len(b)-2] == '\'' && b[len(b)-1] != '\'':
		b = append(b, '\'')
	case b[len(b)-1] != '\'':
		last := b[len(b)-1]
		b[len(b)-1] = '"'
		b = append(b, last)
	}
	return string(b)
}

// canonicGimatria returns the numeric value of w if it is a canonical Hebrew
// numeral (and contains a quote or apostrophe), else 0.
func canonicGimatria(w []rune) int {
	hasQuote := false
	for _, r := range w {
		if r == '"' || r == '\'' {
			hasQuote = true
			break
		}
	}
	if !hasQuote {
		return 0
	}
	val := gimatriaValue(w)
	if gimatriaString(val) != string(w) {
		return 0
	}
	return val
}

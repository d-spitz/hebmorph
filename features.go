package hebmorph

import "strings"

// Morphological description bitmask, ported from hspell's dmask.c. A word's
// "dmask" packs its part of speech, gender, person, number, tense and any
// pronominal suffix into one integer.
const (
	dNoun     = 1
	dVerb     = 2
	dAdj      = 3
	dTypeMask = 3

	dMasculine = 4
	dFeminine  = 8

	dFirst   = 16
	dSecond  = 32
	dThird   = 48
	dGufMask = 48

	dSingular = 64
	dDual     = 128
	dPlural   = 192
	dNumMask  = 192

	dInfinitive  = 256
	dBInfinitive = 1536
	dPast        = 512
	dPresent     = 768
	dFuture      = 1024
	dImperative  = 1280
	dTenseMask   = 1792

	dOMasculine  = 2048
	dOFeminine   = 4096
	dOGenderMask = 6144

	dOFirst   = 8192
	dOSecond  = 16384
	dOThird   = 24576
	dOGufMask = 24576

	dOSingular = 32768
	dODual     = 65536
	dOPlural   = 98304
	dONumMask  = 98304

	dOMask    = 129024
	dOSmichut = 131072
	dSpecNoun = 262144
)

// Prefix specifiers: the set of prefix categories a base word accepts.
const (
	psAll    = 63
	psB      = 1
	psL      = 2
	psVerb   = 4
	psNonDef = 8
	psImper  = 16
)

// prefixSpecifier reports which prefix categories a word with the given dmask
// accepts (ported from linginfo.c's linginfo_dmask2ps). A split's readings are
// filtered to those whose specifier intersects the prefix's mask.
func prefixSpecifier(dmask int) int {
	switch dmask & dTypeMask {
	case dVerb:
		switch {
		case dmask&dTenseMask == dInfinitive:
			return psL
		case dmask&dTenseMask == dBInfinitive:
			return psB
		case dmask&dTenseMask == dImperative:
			return psImper
		case dmask&dTenseMask != dPresent:
			return psVerb
		case dmask&dOSmichut != 0 || dmask&dOMask != 0:
			return psNonDef
		default:
			return psAll
		}
	case dNoun, dAdj:
		if dmask&dOSmichut != 0 || dmask&dOMask != 0 || dmask&dSpecNoun != 0 {
			return psNonDef
		}
		return psAll
	default:
		return psAll
	}
}

// Features is the decoded, language-neutral morphology of a reading.
type Features struct {
	PartOfSpeech string      `json:"part_of_speech,omitempty"`
	Gender       string      `json:"gender,omitempty"`
	Person       string      `json:"person,omitempty"`
	Number       string      `json:"number,omitempty"`
	Tense        string      `json:"tense,omitempty"`
	Construct    bool        `json:"construct,omitempty"`   // סמיכות
	ProperNoun   bool        `json:"proper_noun,omitempty"` // שם פרטי
	Possessive   *Possessive `json:"possessive,omitempty"`  // pronominal suffix
}

// Possessive is a pronominal (possessive/object) suffix.
type Possessive struct {
	Gender string `json:"gender,omitempty"`
	Person string `json:"person,omitempty"`
	Number string `json:"number,omitempty"`
}

func decodeFeatures(dmask int) Features {
	var f Features
	switch dmask & dTypeMask {
	case dNoun:
		f.PartOfSpeech = "noun"
	case dVerb:
		f.PartOfSpeech = "verb"
	case dAdj:
		f.PartOfSpeech = "adjective"
	}
	switch {
	case dmask&dMasculine != 0:
		f.Gender = "masculine"
	case dmask&dFeminine != 0:
		f.Gender = "feminine"
	}
	switch dmask & dGufMask {
	case dFirst:
		f.Person = "1"
	case dSecond:
		f.Person = "2"
	case dThird:
		f.Person = "3"
	}
	switch dmask & dNumMask {
	case dSingular:
		f.Number = "singular"
	case dDual:
		f.Number = "dual"
	case dPlural:
		f.Number = "plural"
	}
	switch dmask & dTenseMask {
	case dPast:
		f.Tense = "past"
	case dPresent:
		f.Tense = "present"
	case dFuture:
		f.Tense = "future"
	case dImperative:
		f.Tense = "imperative"
	case dInfinitive, dBInfinitive:
		f.Tense = "infinitive"
	}
	f.ProperNoun = dmask&dSpecNoun != 0
	f.Construct = dmask&dOSmichut != 0
	if dmask&dOMask != 0 {
		p := &Possessive{}
		switch dmask & dOGenderMask {
		case dOMasculine:
			p.Gender = "masculine"
		case dOFeminine:
			p.Gender = "feminine"
		}
		switch dmask & dOGufMask {
		case dOFirst:
			p.Person = "1"
		case dOSecond:
			p.Person = "2"
		case dOThird:
			p.Person = "3"
		}
		switch dmask & dONumMask {
		case dOSingular:
			p.Number = "singular"
		case dODual:
			p.Number = "dual"
		case dOPlural:
			p.Number = "plural"
		}
		f.Possessive = p
	}
	return f
}

// hebrewDesc renders the traditional hspell Hebrew description string
// (e.g. "פ,נ,3,יחיד,עבר"), kept for readers who want the native notation.
// Ported from linginfo.c's dmask2text.
func hebrewDesc(dmask int) string {
	var b strings.Builder
	switch dmask & dTypeMask {
	case dNoun:
		b.WriteString("ע")
	case dVerb:
		b.WriteString("פ")
	case dAdj:
		b.WriteString("ת")
	case 0:
		b.WriteString("x")
	}
	if dmask&dMasculine != 0 {
		b.WriteString(",ז")
	}
	if dmask&dFeminine != 0 {
		b.WriteString(",נ")
	}
	switch dmask & dGufMask {
	case dFirst:
		b.WriteString(",1")
	case dSecond:
		b.WriteString(",2")
	case dThird:
		b.WriteString(",3")
	}
	switch dmask & dNumMask {
	case dSingular:
		b.WriteString(",יחיד")
	case dDual:
		b.WriteString(",זוגי")
	case dPlural:
		b.WriteString(",רבים")
	}
	switch dmask & dTenseMask {
	case dPast:
		b.WriteString(",עבר")
	case dPresent:
		b.WriteString(",הווה")
	case dFuture:
		b.WriteString(",עתיד")
	case dImperative:
		b.WriteString(",ציווי")
	case dInfinitive:
		b.WriteString(",מקור")
	case dBInfinitive:
		b.WriteString(",מקור,ב")
	}
	if dmask&dSpecNoun != 0 {
		b.WriteString(",פרטי")
	}
	if dmask&dOSmichut != 0 {
		b.WriteString(",סמיכות")
	}
	if dmask&dOMask != 0 {
		b.WriteString(",כינוי/")
		switch dmask & dOGenderMask {
		case dOMasculine:
			b.WriteString("ז")
		case dOFeminine:
			b.WriteString("נ")
		}
		switch dmask & dOGufMask {
		case dOFirst:
			b.WriteString(",1")
		case dOSecond:
			b.WriteString(",2")
		case dOThird:
			b.WriteString(",3")
		}
		switch dmask & dONumMask {
		case dOSingular:
			b.WriteString(",יחיד")
		case dODual:
			b.WriteString(",זוגי")
		case dOPlural:
			b.WriteString(",רבים")
		}
	}
	return b.String()
}

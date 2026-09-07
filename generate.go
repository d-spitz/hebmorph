package hebmorph

// Regenerate the embedded UTF-8 dictionary from hspell's original ISO-8859-8
// data files (kept under internal/gen/source for provenance and reproducibility).
//
//go:generate go run ./internal/gen -src internal/gen/source -out data/hebrew.dict.gz

// Regenerate the embedded English gloss table from the two verified
// translation sets. It joins on Hebrew text, so it runs after the dictionary
// above.
//
//go:generate go run ./internal/gen/translations -stems internal/gen/translate/translations.verified.jsonl -misc internal/gen/translate/misc.verified.jsonl -dict data/hebrew.dict.gz -out data/translations.json.gz

// Regenerate the embedded Modern Hebrew frequency table, which decides the
// order a word's readings come back in. It joins on Hebrew text too, so it
// also runs after the dictionary.
//
//go:generate go run ./internal/gen/frequencies -src internal/gen/translate/lemma_frequencies.json -dict data/hebrew.dict.gz -out data/frequencies.json.gz

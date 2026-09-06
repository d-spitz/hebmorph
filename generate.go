package hebmorph

// Regenerate the embedded UTF-8 dictionary from hspell's original ISO-8859-8
// data files (kept under internal/gen/source for provenance and reproducibility).
//
//go:generate go run ./internal/gen -src internal/gen/source -out data/hebrew.dict.gz

// Regenerate the embedded English gloss table from the verified translation
// set. It joins on stem text, so it runs after the dictionary above.
//
//go:generate go run ./internal/gen/translations -src internal/gen/translate/translations.verified.jsonl -dict data/hebrew.dict.gz -out data/translations.gz

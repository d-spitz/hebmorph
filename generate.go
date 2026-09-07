package hebmorph

// Regenerate the embedded dictionary from hspell's original ISO-8859-8 data
// files (kept under internal/gen/source for provenance and reproducibility),
// plus the spellings in internal/gen/additions.json that hspell rejects but
// Modern Hebrew uses.
//
//go:generate go run ./internal/gen -src internal/gen/source -add internal/gen/additions.json -out data/dictionary.json.gz

// Regenerate the embedded sense table: every distinct word the dictionary can
// mean, with its English and frequency rank, and the lemma each reading
// resolves to. It joins on Hebrew text, so it runs after the dictionary.
//
//go:generate go run ./internal/gen/lemmas -dict data/dictionary.json.gz -out data/lemmas.json.gz

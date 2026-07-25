// Command hebmorph reads Hebrew words, one per line from stdin, and writes the
// morphological analysis of each as a JSON object (one per line).
//
//	echo 'מלכה' | hebmorph
//	hebmorph < words.txt > analyses.jsonl
//
// With -pretty, each analysis is indented.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"hebmorph"
)

func main() {
	pretty := flag.Bool("pretty", false, "indent JSON output")
	flag.Parse()

	a, err := hebmorph.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "hebmorph:", err)
		os.Exit(1)
	}

	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	enc := json.NewEncoder(out)
	if *pretty {
		enc.SetIndent("", "  ")
	}
	for in.Scan() {
		line := in.Text()
		if line == "" {
			continue
		}
		if err := enc.Encode(a.Analyze(line)); err != nil {
			fmt.Fprintln(os.Stderr, "hebmorph:", err)
			os.Exit(1)
		}
	}
	if err := in.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "hebmorph:", err)
		os.Exit(1)
	}
}

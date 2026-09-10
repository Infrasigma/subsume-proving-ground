package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	count := flag.Int("count", 5000, "number of transitions")
	mode := flag.String("mode", modeCompositional, "generation mode: isolated_rules or compositional_ood")
	seed := flag.Int64("seed", 99, "deterministic generation seed")
	out := flag.String("out", "dataset/v8_compositional_smoke.bin", "output dataset path")
	flag.Parse()

	if err := Generate(*out, *count, *mode, *seed); err != nil {
		fmt.Fprintf(os.Stderr, "recorder: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("generated %d records mode=%s seed=%d out=%s\n", *count, *mode, *seed, *out)
}

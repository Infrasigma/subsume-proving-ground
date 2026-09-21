package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Gate struct {
	ID string
	Name string
	Status string
	Evidence string
	Interpretation string
}

type Run struct {
	Commit string
	Verdict string
	Gates []Gate
}

func main() {
	path := "asi_forge/artifacts/ASI_FORGE_RUN.json"
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("REPAIR_STATUS=NO_FORGE_REPORT")
		return
	}
	var r Run
	if err := json.Unmarshal(b, &r); err != nil {
		fmt.Println("REPAIR_STATUS=INVALID_FORGE_REPORT")
		return
	}
	for _, g := range r.Gates {
		if g.Status != "PASS" {
			fmt.Printf("REPAIR_GATE=%s\n", g.ID)
			fmt.Printf("REPAIR_NAME=%s\n", g.Name)
			fmt.Printf("REPAIR_STATUS=%s\n", g.Status)
			fmt.Printf("REPAIR_EVIDENCE=%s\n", g.Evidence)
			fmt.Printf("REPAIR_INTERPRETATION=%s\n", g.Interpretation)
			break
		}
	}
	fmt.Printf("CURRENT_COMMIT=%s\nCURRENT_VERDICT=%s\n", r.Commit, r.Verdict)
}

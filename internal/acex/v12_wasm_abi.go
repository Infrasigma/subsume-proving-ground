package acex

import "fmt"

// BuildV12RelationalCandidates is the host-side ABI marshaller. It performs no
// learned decision or pattern matching: it only converts a RelationalState
// neighbourhood into the fixed raw adjacency-matrix ABI consumed by decide().
func BuildV12RelationalCandidates(state RelationalState, actions []string, out *[V12DecisionMaxCandidates]V12CandidateMatrix) (int, error) {
	if out == nil {
		return 0, fmt.Errorf("nil V12 candidate buffer")
	}
	if len(actions) == 0 || len(actions) > V12DecisionMaxCandidates {
		return 0, fmt.Errorf("action count out of bounds: %d", len(actions))
	}
	for i := range out {
		out[i] = V12CandidateMatrix{}
	}
	for i, action := range actions {
		if err := buildV12RelationalCandidate(state, action, &out[i]); err != nil {
			return 0, fmt.Errorf("candidate %d (%q): %w", i, action, err)
		}
	}
	return len(actions), nil
}

func buildV12RelationalCandidate(state RelationalState, root string, out *V12CandidateMatrix) error {
	// The V12 matcher is bounded to a 16-node candidate neighbourhood and uses
	// root index 0. Discovery here is deliberately allocation-free and treats
	// graph adjacency as undirected only for neighbourhood discovery; edge
	// direction is preserved in the matrix rows below.
	var ids [V12DecisionMaxNodes]string
	var distance [V12DecisionMaxNodes]uint8
	count := 0

	rootPresent := false
	for _, node := range state.Nodes {
		if node.ID == root {
			rootPresent = true
			break
		}
	}
	if !rootPresent {
		return nil
	}

	ids[0] = root
	distance[0] = 0
	count = 1

	for head := 0; head < count; head++ {
		if distance[head] >= 3 {
			continue
		}
		cur := ids[head]
		for _, edge := range state.Edges {
			next := ""
			if edge.From == cur {
				next = edge.To
			} else if edge.To == cur {
				next = edge.From
			} else {
				continue
			}
			if next == "" {
				continue
			}
			found := false
			for i := 0; i < count; i++ {
				if ids[i] == next {
					found = true
					break
				}
			}
			if found {
				continue
			}
			if count >= V12DecisionMaxNodes {
				return fmt.Errorf("candidate neighbourhood exceeds %d nodes", V12DecisionMaxNodes)
			}
			ids[count] = next
			distance[count] = distance[head] + 1
			count++
		}
	}

	for _, edge := range state.Edges {
		from := -1
		to := -1
		for i := 0; i < count; i++ {
			if ids[i] == edge.From {
				from = i
			}
			if ids[i] == edge.To {
				to = i
			}
		}
		if from >= 0 && to >= 0 && from < V12DecisionMaxNodes && to < V12DecisionMaxNodes {
			out.Rows[from] |= uint32(1) << uint32(to)
		}
	}
	return nil
}

package ace

import "errors"

type InstalledMethodRegistry struct {
	Methods []AcquisitionMethodArtifact
	Trace   []string
}

func (r *InstalledMethodRegistry) Install(m AcquisitionMethodArtifact) error {
	if m.ID == "" || m.Name == "" || m.Procedure == "" || m.Artifact == "" {
		return errors.New("cannot install incomplete acquisition method")
	}
	for _, old := range r.Methods {
		if old.ID == m.ID {
			return nil
		}
	}
	r.Methods = append(r.Methods, m)
	r.Trace = append(r.Trace, "installed:"+m.Name)
	return nil
}

func (r *InstalledMethodRegistry) Apply(spec CapabilitySpecification) ([]ArchitectureCandidate, error) {
	cs, err := (UniversalMechanismSearch{}).SearchMechanisms(spec, spec.ResourceLimits)
	if err != nil {
		return nil, err
	}
	before := make([]string, len(cs))
	for i := range cs { before[i] = cs[i].Mechanism }
	for i := len(r.Methods)-1; i >= 0; i-- {
		var next []ArchitectureCandidate
		next, err = executeAcquisitionMethod(r.Methods[i], spec)
		if err != nil { return nil, err }
		cs = next
		after := make([]string, len(cs))
		for j := range cs { after[j] = cs[j].Mechanism }
		r.Trace = append(r.Trace, methodTrace(r.Methods[i], before, after))
		before = after
	}
	return cs, nil
}

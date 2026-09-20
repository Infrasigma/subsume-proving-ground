package ace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type persistedAcquisitionPolicy struct {
	Version uint64           `json:"version"`
	Policy  AcquisitionPolicy `json:"policy"`
}

type PersistentAcquisitionPolicy struct {
	Path string
	Data persistedAcquisitionPolicy
}

func NewPersistentAcquisitionPolicy(path string) (*PersistentAcquisitionPolicy, error) {
	r := &PersistentAcquisitionPolicy{Path: path}
	b, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(b, &r.Data); err != nil {
			return nil, err
		}
		return r, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return r, nil
}

func (r *PersistentAcquisitionPolicy) Restore(dst *AcquisitionPolicy) error {
	if r == nil || dst == nil {
		return errors.New("nil acquisition policy destination")
	}
	*dst = r.Data.Policy
	return nil
}

func (r *PersistentAcquisitionPolicy) Save(src *AcquisitionPolicy) error {
	if r == nil || src == nil {
		return errors.New("nil acquisition policy source")
	}
	r.Data.Policy = *src
	r.Data.Version++
	if r.Path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(r.Path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(r.Data, "", "  ")
	if err != nil {
		return err
	}
	tmp := r.Path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, r.Path)
}

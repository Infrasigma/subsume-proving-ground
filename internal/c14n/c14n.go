package c14n

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Canonicalize implements AACR's current restricted JSON canonicalization profile.
// It deliberately rejects floating-point forms and accepts only signed int64
// integers. This is a v0.1 fail-closed bridge until the production RFC 8785
// implementation is frozen.
func Canonicalize(v any) ([]byte, error) {
	var b bytes.Buffer
	if err := write(&b, v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func write(b *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case string:
		q, err := json.Marshal(x)
		if err != nil {
			return err
		}
		b.Write(q)
	case json.Number:
		s := x.String()
		if strings.ContainsAny(s, ".eE") {
			return errors.New("floating-point JSON numbers are rejected by AACR canonical profile v0.1")
		}
		if _, err := strconv.ParseInt(s, 10, 64); err != nil {
			return fmt.Errorf("invalid signed int64: %w", err)
		}
		b.WriteString(s)
	case float64:
		return errors.New("floating-point JSON numbers are rejected by AACR canonical profile v0.1")
	case []any:
		b.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := write(b, item); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		// Interim v0.1 profile: deterministic Go UTF-8 byte-order sorting.
		// This is deliberately not claimed as full RFC 8785/JCS compliance.
		slices.Sort(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return err
			}
			b.Write(kb)
			b.WriteByte(':')
			if err := write(b, x[k]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		return fmt.Errorf("unsupported JSON value type %T", v)
	}
	return nil
}

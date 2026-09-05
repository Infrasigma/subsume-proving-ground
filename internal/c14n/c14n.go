package c14n

import (
    "bytes"
    "encoding/json"
    "errors"
    "fmt"
    "strconv"
    "strings"
)

// Canonicalize implements AACR's current restricted JSON canonicalization profile.
// It deliberately rejects floats and non-finite numeric forms. This is a v0.1
// fail-closed bridge until the production RFC 8785 implementation is frozen.
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
        if x { b.WriteString("true") } else { b.WriteString("false") }
    case string:
        q, err := json.Marshal(x)
        if err != nil { return err }
        b.Write(q)
    case json.Number:
        s := x.String()
        if strings.ContainsAny(s, ".eE") {
            return errors.New("floating-point JSON numbers are rejected by AACR canonical profile v0.1")
        }
        if _, err := strconv.ParseInt(s, 10, 64); err != nil {
            return fmt.Errorf("invalid integer: %w", err)
        }
        b.WriteString(s)
    case float64:
        return errors.New("floating-point JSON numbers are rejected by AACR canonical profile v0.1")
    case []any:
        b.WriteByte('[')
        for i, item := range x {
            if i > 0 { b.WriteByte(',') }
            if err := write(b, item); err != nil { return err }
        }
        b.WriteByte(']')
    case map[string]any:
        keys := make([]string, 0, len(x))
        for k := range x { keys = append(keys, k) }
        // NOTE: this lexical sort is the interim restricted profile, not a claim
        // of complete RFC 8785/JCS compliance.
        sortStrings(keys)
        b.WriteByte('{')
        for i, k := range keys {
            if i > 0 { b.WriteByte(',') }
            kb, _ := json.Marshal(k)
            b.Write(kb)
            b.WriteByte(':')
            if err := write(b, x[k]); err != nil { return err }
        }
        b.WriteByte('}')
    default:
        return fmt.Errorf("unsupported JSON value type %T", v)
    }
    return nil
}

func sortStrings(a []string) {
    for i := 1; i < len(a); i++ {
        key := a[i]
        j := i - 1
        for j >= 0 && a[j] > key {
            a[j+1] = a[j]
            j--
        }
        a[j+1] = key
    }
}

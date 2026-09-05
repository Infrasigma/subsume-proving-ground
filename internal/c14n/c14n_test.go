package c14n

import (
    "encoding/json"
    "io"
    "testing"
)

func TestCanonicalizeDeterministic(t *testing.T) {
    var v any
    dec := json.NewDecoder(stringsReader(`{"b":2,"a":1,"arr":[true,null,"x"]}`))
    dec.UseNumber()
    if err := dec.Decode(&v); err != nil { t.Fatal(err) }
    got, err := Canonicalize(v)
    if err != nil { t.Fatal(err) }
    want := `{"a":1,"arr":[true,null,"x"],"b":2}`
    if string(got) != want { t.Fatalf("got %s want %s", got, want) }
}

func TestRejectFloat(t *testing.T) {
    var v any
    dec := json.NewDecoder(stringsReader(`{"n":1.5}`))
    dec.UseNumber()
    if err := dec.Decode(&v); err != nil { t.Fatal(err) }
    if _, err := Canonicalize(v); err == nil { t.Fatal("expected float rejection") }
}

type strReader struct { s string; i int }
func (r *strReader) Read(p []byte) (int, error) {
    if r.i >= len(r.s) { return 0, io.EOF }
    n := copy(p, r.s[r.i:])
    r.i += n
    return n, nil
}
func stringsReader(s string) *strReader { return &strReader{s:s} }

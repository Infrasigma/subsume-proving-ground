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
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	got, err := Canonicalize(v)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":1,"arr":[true,null,"x"],"b":2}`
	if string(got) != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestCanonicalizeUnicodeKeyOrder(t *testing.T) {
	var v any
	dec := json.NewDecoder(stringsReader(`{"z":1,"é":2,"😀":3,"a":4}`))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	got, err := Canonicalize(v)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":4,"z":1,"é":2,"😀":3}`
	if string(got) != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestAcceptSignedInt64Bounds(t *testing.T) {
	for _, input := range []string{`-9223372036854775808`, `9223372036854775807`} {
		var v any
		dec := json.NewDecoder(stringsReader(input))
		dec.UseNumber()
		if err := dec.Decode(&v); err != nil {
			t.Fatal(err)
		}
		if _, err := Canonicalize(v); err != nil {
			t.Fatalf("%s rejected: %v", input, err)
		}
	}
}

func TestRejectIntegerOverflow(t *testing.T) {
	var v any
	dec := json.NewDecoder(stringsReader(`9223372036854775808`))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	if _, err := Canonicalize(v); err == nil {
		t.Fatal("expected int64 overflow rejection")
	}
}

func TestRejectFloat(t *testing.T) {
	var v any
	dec := json.NewDecoder(stringsReader(`{"n":1.5}`))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	if _, err := Canonicalize(v); err == nil {
		t.Fatal("expected float rejection")
	}
}

type strReader struct {
	s string
	i int
}

func (r *strReader) Read(p []byte) (int, error) {
	if r.i >= len(r.s) {
		return 0, io.EOF
	}
	n := copy(p, r.s[r.i:])
	r.i += n
	return n, nil
}

func stringsReader(s string) *strReader { return &strReader{s: s} }

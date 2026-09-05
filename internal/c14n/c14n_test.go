package c14n

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func decodeNumber(t *testing.T, input string) any {
	t.Helper()
	var v any
	dec := json.NewDecoder(strings.NewReader(input))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestCanonicalizeDeterministic(t *testing.T) {
	v := decodeNumber(t, `{"b":2,"a":1,"arr":[true,null,"x"]}`)
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
	v := decodeNumber(t, `{"z":1,"é":2,"a":4}`)
	got, err := Canonicalize(v)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":4,"z":1,"é":2}`
	if string(got) != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestRejectSupplementaryUnicodeObjectKey(t *testing.T) {
	v := decodeNumber(t, `{"😀":1}`)
	if _, err := Canonicalize(v); err == nil {
		t.Fatal("expected supplementary Unicode object key rejection")
	}
}

func TestAcceptSignedInt64Bounds(t *testing.T) {
	for _, input := range []string{`-9223372036854775808`, `9223372036854775807`} {
		if _, err := Canonicalize(decodeNumber(t, input)); err != nil {
			t.Fatalf("%s rejected: %v", input, err)
		}
	}
}

func TestNormalizeNegativeZero(t *testing.T) {
	got, err := Canonicalize(decodeNumber(t, `-0`))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `0` {
		t.Fatalf("got %s want 0", got)
	}
}

func TestRejectIntegerOverflow(t *testing.T) {
	if _, err := Canonicalize(decodeNumber(t, `9223372036854775808`)); err == nil {
		t.Fatal("expected int64 overflow rejection")
	}
}

func TestRejectFloat(t *testing.T) {
	if _, err := Canonicalize(decodeNumber(t, `{"n":1.5}`)); err == nil {
		t.Fatal("expected float rejection")
	}
}

func TestRejectDeepNesting(t *testing.T) {
	var b strings.Builder
	for i := 0; i < maxDepth+1; i++ {
		b.WriteByte('[')
	}
	b.WriteString(`0`)
	for i := 0; i < maxDepth+1; i++ {
		b.WriteByte(']')
	}

	if _, err := Canonicalize(decodeNumber(t, b.String())); !errors.Is(err, ErrExceededMaxDepth) {
		t.Fatalf("got %v want %v", err, ErrExceededMaxDepth)
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

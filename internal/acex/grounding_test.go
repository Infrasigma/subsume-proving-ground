package acex

import "testing"

func TestV2RawGroundingAndRenaming(t *testing.T) {
	rawA:=[]byte(`{
		"entities":[
			{"id":"a","kind":"agent","attrs":{"role":"source"}},
			{"id":"b","kind":"agent","attrs":{"role":"middle"}},
			{"id":"c","kind":"agent","attrs":{"role":"sink"}}
		],
		"relations":[
			{"from":"a","to":"b","kind":"causes"},
			{"from":"b","to":"c","kind":"causes"}
		],
		"actions":["inspect","act"]
	}`)
	rawB:=[]byte(`{
		"entities":[
			{"id":"x7","kind":"agent","attrs":{"role":"source"}},
			{"id":"x2","kind":"agent","attrs":{"role":"middle"}},
			{"id":"x9","kind":"agent","attrs":{"role":"sink"}}
		],
		"relations":[
			{"from":"x2","to":"x9","kind":"causes"},
			{"from":"x7","to":"x2","kind":"causes"}
		],
		"actions":["act","inspect"]
	}`)
	g:=GroundingRuntime{Grounder:JSONGrounder{}}
	a,err:=g.Observe(rawA)
	if err!=nil { t.Fatal(err) }
	b,err:=g.Observe(rawB)
	if err!=nil { t.Fatal(err) }
	if a.Digest!=b.Digest {
		t.Fatalf("raw symbol renaming changed grounded invariant: %s != %s",a.Digest,b.Digest)
	}
	if len(a.Actions)!=2 || len(b.Actions)!=2 {
		t.Fatal("grounding lost action affordances")
	}
}

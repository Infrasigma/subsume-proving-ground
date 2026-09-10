package ace

import "testing"

func TestDeepRecursiveCompoundingEvidence(t *testing.T){m,e:=RunDeepRecursiveProtocolV2();if e!=nil{t.Fatal(e)};if m["verified"]!=1{t.Fatalf("not verified: %#v",m)};if m["R"]>=1{t.Fatalf("expected R<1: %#v",m)}}

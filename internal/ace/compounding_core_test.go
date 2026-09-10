package ace

import "testing"

func TestRecursiveCapabilityCompoundingCore(t *testing.T){m,e:=RunRecursiveCapabilityProtocolV3();if e!=nil{t.Fatal(e)};if m["verified"]!=1{t.Fatalf("not verified: %#v",m)};if m["R"]>=1{t.Fatalf("R not below one: %#v",m)}}
func TestReplicatedCapabilityCompoundingCore(t *testing.T){m,e:=RunReplicatedCompoundingV2(12);if e!=nil{t.Fatal(e)};if m["wins"]!=12||m["all_verified"]!=1{t.Fatalf("replication failure: %#v",m)};if m["mean_R"]>=1{t.Fatalf("mean R not below one: %#v",m)}}

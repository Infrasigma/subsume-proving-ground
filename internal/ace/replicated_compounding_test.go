package ace

import "testing"

func TestReplicatedCapabilityCompounding(t *testing.T){m,e:=RunReplicatedCompoundingProtocol(12);if e!=nil{t.Fatal(e)};if m["all_verified"]!=1||m["wins"]!=12{t.Fatalf("replication failure: %#v",m)};if m["mean_R"]>=1{t.Fatalf("replication did not compound: %#v",m)}}

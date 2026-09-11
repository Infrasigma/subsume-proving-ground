package ace

import("path/filepath";"testing")

func TestExecutableMethodPersistsAcrossRestart(t *testing.T){path:=filepath.Join(t.TempDir(),"library.json");p,err:=OpenPersistentSearchLibrary(path);if err!=nil{t.Fatal(err)};m:=ExecutableAcquisitionMethod{ID:"persisted-m1",Instructions:[]MethodInstruction{{Op:"search",Arg:"search:branching"}},Verifier:"independent-heldout",Version:1};if err=p.RegisterMethod(m);err!=nil{t.Fatal(err)};q,err:=OpenPersistentSearchLibrary(path);if err!=nil{t.Fatal(err)};got,ok:=q.Library.Methods[m.ID];if !ok||len(got.Instructions)!=1||got.Instructions[0]!=m.Instructions[0]{t.Fatalf("method did not survive restart: %#v",q.Library.Methods)}}

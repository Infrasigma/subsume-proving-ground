package ace

import("encoding/json";"errors";"os";"path/filepath")

type PersistentSearchLibrary struct{Path string;Library SearchLibrary}
func OpenPersistentSearchLibrary(path string)(*PersistentSearchLibrary,error){if path==""{return nil,errors.New("library path required")};p:=&PersistentSearchLibrary{Path:path,Library:*NewSearchLibrary()};b,e:=os.ReadFile(path);if os.IsNotExist(e){return p,nil};if e!=nil{return nil,e};if e=json.Unmarshal(b,&p.Library);e!=nil{return nil,e};if p.Library.Primitives==nil{p.Library.Primitives=map[string]SearchPrimitive{}};if p.Library.Methods==nil{p.Library.Methods=map[string]ExecutableAcquisitionMethod{}};if p.Library.Abstractions==nil{p.Library.Abstractions=map[string]ExecutableAcquisitionMethod{}};return p,nil}
func(p *PersistentSearchLibrary)flush()error{b,e:=json.MarshalIndent(p.Library,"","  ");if e!=nil{return e};if e=os.MkdirAll(filepath.Dir(p.Path),0755);e!=nil{return e};tmp:=p.Path+".tmp";if e=os.WriteFile(tmp,b,0600);e!=nil{return e};return os.Rename(tmp,p.Path)}
func(p *PersistentSearchLibrary)RegisterMethod(m ExecutableAcquisitionMethod)error{if e:=p.Library.RegisterMethod(m);e!=nil{return e};return p.flush()}
func(p *PersistentSearchLibrary)RegisterPrimitive(s SearchPrimitive)error{if e:=p.Library.RegisterPrimitive(s);e!=nil{return e};return p.flush()}

package ace

import("encoding/json";"errors";"os";"path/filepath")
type persistedMethods struct{Version uint64 `json:"version"`;Methods []AcquisitionMethodArtifact `json:"methods"`}
type PersistentMethodRegistry struct{Path string;Data persistedMethods}
func NewPersistentMethodRegistry(path string)(*PersistentMethodRegistry,error){r:=&PersistentMethodRegistry{Path:path};b,e:=os.ReadFile(path);if e==nil{if e=json.Unmarshal(b,&r.Data);e!=nil{return nil,e}}else if !errors.Is(e,os.ErrNotExist){return nil,e};return r,nil}
func(r *PersistentMethodRegistry)Methods()[]AcquisitionMethodArtifact{return append([]AcquisitionMethodArtifact(nil),r.Data.Methods...)}
func(r *PersistentMethodRegistry)Install(m AcquisitionMethodArtifact)error{if m.ID==""||m.Artifact==""{return errors.New("incomplete method")};for _,x:=range r.Data.Methods{if x.ID==m.ID{return nil}};if _,e:=decodeAcquisitionProcedure(m.Artifact);e!=nil{return e};r.Data.Methods=append(r.Data.Methods,m);r.Data.Version++;if r.Path==""{return nil};if e:=os.MkdirAll(filepath.Dir(r.Path),0755);e!=nil{return e};b,e:=json.MarshalIndent(r.Data,"","  ");if e!=nil{return e};tmp:=r.Path+".tmp";if e=os.WriteFile(tmp,b,0600);e!=nil{return e};return os.Rename(tmp,r.Path)}
func(r *PersistentMethodRegistry)Restore(dst *InstalledMethodRegistry)error{if dst==nil{return errors.New("nil destination")};for _,m:=range r.Data.Methods{if e:=dst.Install(m);e!=nil{return e}};return nil}

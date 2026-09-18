package t2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

type Arm string
const ( ArmMPlus Arm="M+"; ArmMMinus Arm="M-"; ArmReimpl Arm="M^reimpl"; ArmFresh Arm="A0^fresh" )

type ResourceBudget struct { TokenBudget int64; CPUTimeMS int64; WallTimeMS int64 }
type ArmRequest struct { ProtocolVersion string; StudyID string; Arm Arm; TaskFile string; ResultFile string; Budget ResourceBudget }
type ArmUsage struct { TokensIn int64; TokensOut int64; CPUTimeMS int64; WallTimeMS int64 }
type Prediction struct { TaskID string; Answer json.RawMessage }
type ArmResponse struct { Arm Arm; Predictions []Prediction; Usage ArmUsage; StateHash string }

type CostModel struct { TokenWeight float64; CPUTimeMSWeight float64 }

type ImmutableArmConfig struct { Arm Arm; StudyID string; Executable string; ExpectedSHA256 string; Args []string; Tasks []PublicTask; Budget ResourceBudget; CostModel CostModel }

type AdapterEvidence struct { Arm Arm; ExecutableSHA256 string; WorkspaceHash string; StateHash string; Usage ArmUsage; CompositeCost float64; Capability float64; StructuralX float64 }

func SHA256File(path string)(string,error){f,err:=os.Open(path);if err!=nil{return "",err};defer f.Close();h:=sha256.New();if _,err:=io.Copy(h,f);err!=nil{return "",err};return hex.EncodeToString(h.Sum(nil)),nil}

func RunImmutableArm(ctx context.Context,cfg ImmutableArmConfig,scored []ScoredTask)(AdapterEvidence,error){
	if runtime.GOOS!="linux"{return AdapterEvidence{},errors.New("immutable T2 sandbox requires Linux")}
	if cfg.Executable==""||cfg.ExpectedSHA256==""{return AdapterEvidence{},errors.New("executable hash required")}
	got,err:=SHA256File(cfg.Executable);if err!=nil{return AdapterEvidence{},err};if got!=cfg.ExpectedSHA256{return AdapterEvidence{},fmt.Errorf("executable hash mismatch: got %s want %s",got,cfg.ExpectedSHA256)}
	if len(scored)==0{return AdapterEvidence{},errors.New("empty scored task set")}
	work,err:=os.MkdirTemp("","t2-arm-");if err!=nil{return AdapterEvidence{},err};defer os.RemoveAll(work)
	input:=filepath.Join(work,"tasks.jsonl");request:=filepath.Join(work,"request.json");result:=filepath.Join(work,"result.json")
	if err:=writePublicTasks(input,cfg.Tasks);err!=nil{return AdapterEvidence{},err};req:=ArmRequest{ProtocolVersion:ProtocolVersion,StudyID:cfg.StudyID,Arm:cfg.Arm,TaskFile:"/input/tasks.jsonl",ResultFile:"/work/result.json",Budget:cfg.Budget};if err:=writeJSONFile(request,req,0600);err!=nil{return AdapterEvidence{},err}
	bwrap,err:=exec.LookPath("bwrap");if err!=nil{return AdapterEvidence{},errors.New("bubblewrap is required")}
	args:=[]string{"--die-with-parent","--new-session","--clearenv","--unshare-net","--unshare-pid","--unshare-ipc","--unshare-uts","--unshare-cgroup","--dir","/input","--dir","/work","--dev","/dev","--proc","/proc","--tmpfs","/tmp","--ro-bind",cfg.Executable,"/arm/runner","--ro-bind",input,"/input/tasks.jsonl","--ro-bind",request,"/input/request.json","--bind",work,"/work","--setenv","T2_STUDY_ID",cfg.StudyID,"--setenv","T2_ARM",string(cfg.Arm),"/arm/runner"}
	args=append(args,cfg.Args...)
	deadline:=time.Duration(cfg.Budget.WallTimeMS)*time.Millisecond;childCtx,cancel:=context.WithTimeout(ctx,deadline);defer cancel();cmd:=exec.CommandContext(childCtx,bwrap,args...);start:=time.Now()
	if err:=cmd.Run();err!=nil{if childCtx.Err()!=nil{return AdapterEvidence{},fmt.Errorf("%s exceeded wall-time budget",cfg.Arm)};return AdapterEvidence{},fmt.Errorf("%s execution failed: %w",cfg.Arm,err)}
	wall:=time.Since(start);if wall.Milliseconds()>cfg.Budget.WallTimeMS{return AdapterEvidence{},fmt.Errorf("%s exceeded wall-time budget",cfg.Arm)}
	b,err:=os.ReadFile(result);if err!=nil{return AdapterEvidence{},fmt.Errorf("%s produced no result file: %w",cfg.Arm,err)}
	var resp ArmResponse;if err:=json.Unmarshal(b,&resp);err!=nil{return AdapterEvidence{},fmt.Errorf("invalid arm response: %w",err)};if resp.Arm!=cfg.Arm{return AdapterEvidence{},errors.New("arm identity mismatch")}
	if resp.Usage.TokensIn<0||resp.Usage.TokensOut<0||resp.Usage.TokensIn+resp.Usage.TokensOut>cfg.Budget.TokenBudget{return AdapterEvidence{},errors.New("token budget exceeded")}
	actualCPU:=int64((cmd.ProcessState.UserTime()+cmd.ProcessState.SystemTime())/time.Millisecond)\n\tresp.Usage.CPUTimeMS=actualCPU\n\tif actualCPU<0||actualCPU>cfg.Budget.CPUTimeMS{return AdapterEvidence{},errors.New("CPU budget exceeded")}
	resp.Usage.WallTimeMS=wall.Milliseconds();if resp.Usage.WallTimeMS>cfg.Budget.WallTimeMS{return AdapterEvidence{},errors.New("wall-time budget exceeded")}
	capability,structural:=scorePredictions(resp.Predictions,scored);cost:=cfg.CostModel.TokenWeight*float64(resp.Usage.TokensIn+resp.Usage.TokensOut)+cfg.CostModel.CPUTimeMSWeight*float64(resp.Usage.CPUTimeMS)
	return AdapterEvidence{Arm:cfg.Arm,ExecutableSHA256:got,WorkspaceHash:SHA256Bytes([]byte("isolated-workspace-v1")),StateHash:resp.StateHash,Usage:resp.Usage,CompositeCost:cost,Capability:capability,StructuralX:structural},nil
}

func writePublicTasks(path string,tasks []PublicTask)error{f,err:=os.OpenFile(path,os.O_WRONLY|os.O_CREATE|os.O_TRUNC,0600);if err!=nil{return err};defer f.Close();e:=json.NewEncoder(f);for _,t:=range tasks{if err:=e.Encode(t);err!=nil{return err}};return nil}
func writeJSONFile(path string,v any,mode os.FileMode)error{b,err:=json.Marshal(v);if err!=nil{return err};return os.WriteFile(path,b,mode)}
func scorePredictions(pred []Prediction,scored []ScoredTask)(float64,float64){exp:=map[string]json.RawMessage{};nov:=map[string]bool{};for _,t:=range scored{exp[t.Public.ID]=t.Oracle;nov[t.Public.ID]=t.Public.Novel};got:=map[string]json.RawMessage{};for _,p:=range pred{got[p.TaskID]=p.Answer};correct,novCorrect,novTotal:=0,0,0;for id,want:=range exp{ans,ok:=got[id];if ok&&string(ans)==string(want){correct++};if nov[id]{novTotal++;if ok&&string(ans)==string(want){novCorrect++}}};if len(exp)==0{return 0,0};structural:=0.0;if novTotal>0{structural=float64(novCorrect)/float64(novTotal)};return float64(correct)/float64(len(exp)),structural}

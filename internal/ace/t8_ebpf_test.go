package ace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestT8KernelGovernanceReceiptPatchAndF0Seal(t *testing.T) {
	rootKey:=bytes.Repeat([]byte{0x42},32)
	runtimeState:=newF0TestRuntime(t)
	gen:=DefaultAutotelicTaskGenerator(); heuristic:=DefaultSearchHeuristicProgram()
	task,err:=gen.GenerateKernelGovernanceTask(context.Background(),&AbstractionLibrary{},&heuristic,8,8);if err!=nil{t.Fatal(err)}
	source:=StaticKernelTelemetrySource{SnapshotValue:KernelTelemetrySnapshot{PID:uint32(os.Getpid()),ObservedAtUnixNanos:time.Now().UnixNano(),CPUPercent:92,RSSBytes:900*1024*1024,TotalAllocBytes:64*1024*1024,MemoryTotalBytes:1024*1024*1024,SchedulerEvents:17,AllocationEvents:9}}
	controller:=DefaultAxonSubstrateController(rootKey);controller.MaxWorkers=8
	execn,err:=controller.RunT8KernelGovernanceCrucible(context.Background(),&runtimeState,task,source);if err!=nil{t.Fatalf("T8 crucible failed: %v",err)}
	if execn.CompletedMiB!=task.PlannedHoldMiB{t.Fatalf("completed=%d want=%d",execn.CompletedMiB,task.PlannedHoldMiB)}
	if execn.GovernancePatch.ToWorkers!=4||execn.GovernancePatch.MaxMemoryMiB!=task.SafeMemoryMiB{t.Fatalf("unexpected patch: %+v",execn.GovernancePatch)}
	if execn.PeakHeldMiB>task.SafeMemoryMiB{t.Fatalf("peak=%d exceeded=%d",execn.PeakHeldMiB,task.SafeMemoryMiB)}
	if err:=execn.KernelReceipt.Verify(rootKey);err!=nil{t.Fatal(err)}
	if err:=execn.GovernancePatch.Validate(task.MaxWorkers);err!=nil{t.Fatal(err)}
	plane:=execn.Sealed.ExecutionControlPlane
	if plane==nil||len(plane.KernelTelemetryReceipts)!=1||len(plane.ResourceGovernancePatches)!=1{t.Fatalf("sealed plane incomplete: %+v",plane)}
	if plane.TraceBackend!=T8EBPFTraceBackend||plane.GovernanceProtocol!=T8EBPFGovernanceProtocol{t.Fatalf("unexpected backend: %+v",plane)}
	if err:=execn.Sealed.VerifyAdmission(execn.Sealed.KMSSignature.PublicKeyB64);err!=nil{t.Fatal(err)}
	tampered:=execn.KernelReceipt;tampered.RSSBytes++
	if err:=tampered.Verify(nil);err==nil{t.Fatal("tampered kernel receipt accepted")}
	tamperedPatch:=execn.GovernancePatch;tamperedPatch.MaxMemoryMiB++
	if err:=tamperedPatch.Validate(task.MaxWorkers);err==nil{t.Fatal("tampered governance patch accepted")}
}

func TestT8GovernedMemoryTaskCannotWidenBudget(t *testing.T) {
	task:=T8KernelGovernanceTask{ID:"t8-budget",InitialWorkers:4,MaxWorkers:4,PlannedHoldMiB:16,SafeMemoryMiB:4,ChunkMiB:1,TriggerPressurePct:80}
	snapshot:=KernelTelemetrySnapshot{PID:uint32(os.Getpid()),ObservedAtUnixNanos:time.Now().UnixNano(),CPUPercent:10,RSSBytes:100*1024*1024,MemoryTotalBytes:1000*1024*1024}
	receipt,err:=SignKernelTelemetryReceipt(snapshot,"axon-worker-01",bytes.Repeat([]byte{0x43},32));if err!=nil{t.Fatal(err)}
	patch,err:=SynthesizeResourceGovernancePatch(snapshot,receipt,4,4,4);if err!=nil{t.Fatal(err)}
	patch.MaxMemoryMiB=8
	if _,_,err:=executeGovernedMemoryTask(context.Background(),task,patch);err==nil{t.Fatal("widened governance budget accepted")}
}

func TestT8LiveEBPFProbeCapability(t *testing.T) {
	if runtime.GOOS!="linux"{t.Skip("Linux-only live eBPF capability")}
	probe:=NewLinuxEBPFProbe()
	ctx,cancel:=context.WithTimeout(context.Background(),3*time.Second);defer cancel()
	if err:=probe.Start(ctx);err!=nil{t.Skipf("live eBPF unavailable in runner: %v",err)}
	defer probe.Stop()
	deadline:=time.Now().Add(200*time.Millisecond)
	for time.Now().Before(deadline){_=sha256.Sum256(make([]byte,256));runtime.Gosched()}
	counts:=probe.Snapshot(uint32(os.Getpid()))
	if counts.SchedulerEvents==0{t.Log("eBPF hook attached but no target-PID scheduler records were observed during the capability window")}
}

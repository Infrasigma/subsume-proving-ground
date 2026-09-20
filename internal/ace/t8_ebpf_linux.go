//go:build linux

package ace

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/metrics"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/rlimit"
)

const (
	t8SchedulerEvent uint32 = 1
	t8PageAllocEvent uint32 = 2
	t8EventSize = 16
)

type t8KernelEvent struct { PID uint32; Kind uint32; Timestamp uint64 }
type KernelEventCounts struct { SchedulerEvents uint64; AllocationEvents uint64 }

type LinuxEBPFProbe struct {
	events *ebpf.Map
	reader *perf.Reader
	programs []*ebpf.Program
	links []link.Link
	mu sync.RWMutex
	counts map[uint32]KernelEventCounts
	once sync.Once
}

func NewLinuxEBPFProbe() *LinuxEBPFProbe { return &LinuxEBPFProbe{counts: make(map[uint32]KernelEventCounts)} }

func t8ProgramSpec(name string, kind uint32, events *ebpf.Map) *ebpf.ProgramSpec {
	ins := asm.Instructions{
		asm.FnGetCurrentPidTgid.Call(),
		asm.Rsh.Imm(asm.R0, 32),
		asm.StoreMem(asm.RFP, -16, asm.R0, asm.Word),
		asm.Mov.Imm(asm.R2, int64(kind)),
		asm.StoreMem(asm.RFP, -12, asm.R2, asm.Word),
		asm.FnKtimeGetNs.Call(),
		asm.StoreMem(asm.RFP, -8, asm.R0, asm.DWord),
		asm.LoadMapPtr(asm.R2, events.FD()),
		asm.LoadImm(asm.R3, 0xffffffff, asm.DWord),
		asm.Mov.Reg(asm.R4, asm.RFP),
		asm.Add.Imm(asm.R4, -16),
		asm.Mov.Imm(asm.R5, t8EventSize),
		asm.FnPerfEventOutput.Call(),
		asm.Mov.Imm(asm.R0, 0),
		asm.Return(),
	}
	return &ebpf.ProgramSpec{Name: name, Type: ebpf.TracePoint, License: "GPL", Instructions: ins}
}

func (p *LinuxEBPFProbe) Start(ctx context.Context) error {
	if p == nil { return errors.New("nil Linux eBPF probe") }
	if ctx == nil { ctx = context.Background() }
	if err := rlimit.RemoveMemlock(); err != nil { return fmt.Errorf("eBPF memlock setup: %w", err) }
	events, err := ebpf.NewMap(&ebpf.MapSpec{Type: ebpf.PerfEventArray, Name: "ace_t8_kernel_events"})
	if err != nil { return fmt.Errorf("eBPF perf event array: %w", err) }
	reader, err := perf.NewReader(events, os.Getpagesize())
	if err != nil { _ = events.Close(); return fmt.Errorf("eBPF perf reader: %w", err) }
	specs := []struct{name,group,event string; kind uint32}{
		{name:"ace_t8_sched_switch", group:"sched", event:"sched_switch", kind:t8SchedulerEvent},
		{name:"ace_t8_page_alloc", group:"kmem", event:"mm_page_alloc", kind:t8PageAllocEvent},
	}
	programs := make([]*ebpf.Program,0,len(specs)); links := make([]link.Link,0,len(specs))
	cleanup := func(){ for _,l := range links { _=l.Close() }; for _,pr := range programs { _=pr.Close() }; _=reader.Close(); _=events.Close() }
	for _, item := range specs {
		prog, err := ebpf.NewProgram(t8ProgramSpec(item.name,item.kind,events))
		if err != nil { cleanup(); return fmt.Errorf("eBPF program %s: %w",item.name,err) }
		tp, err := link.Tracepoint(item.group,item.event,prog,nil)
		if err != nil { _=prog.Close(); cleanup(); return fmt.Errorf("eBPF tracepoint %s:%s: %w",item.group,item.event,err) }
		programs=append(programs,prog); links=append(links,tp)
	}
	p.mu.Lock(); p.events=events; p.reader=reader; p.programs=programs; p.links=links; p.mu.Unlock()
	go p.readLoop()
	go func(){ <-ctx.Done(); _=p.Stop() }()
	return nil
}

func (p *LinuxEBPFProbe) readLoop() {
	for {
		record, err := p.reader.Read()
		if err != nil { return }
		if len(record.RawSample) < t8EventSize { continue }
		event := t8KernelEvent{PID:binary.LittleEndian.Uint32(record.RawSample[0:4]), Kind:binary.LittleEndian.Uint32(record.RawSample[4:8]), Timestamp:binary.LittleEndian.Uint64(record.RawSample[8:16])}
		p.mu.Lock()
		c := p.counts[event.PID]
		if event.Kind==t8SchedulerEvent { c.SchedulerEvents++ } else if event.Kind==t8PageAllocEvent { c.AllocationEvents++ }
		p.counts[event.PID]=c
		p.mu.Unlock()
	}
}

func (p *LinuxEBPFProbe) Snapshot(pid uint32) KernelEventCounts {
	if p == nil { return KernelEventCounts{} }
	p.mu.RLock(); defer p.mu.RUnlock()
	return p.counts[pid]
}

func (p *LinuxEBPFProbe) Stop() error {
	if p == nil { return nil }
	var err error
	p.once.Do(func(){
		p.mu.Lock(); reader:=p.reader; links:=append([]link.Link(nil),p.links...); programs:=append([]*ebpf.Program(nil),p.programs...); events:=p.events; p.mu.Unlock()
		if reader!=nil { err=reader.Close() }
		for _,l:=range links { if e:=l.Close(); err==nil { err=e } }
		for _,pr:=range programs { _=pr.Close() }
		if events!=nil { _=events.Close() }
	})
	return err
}

type LinuxKernelTelemetrySource struct {
	Probe *LinuxEBPFProbe
	mu sync.Mutex
	lastCPU float64
	lastWall time.Time
}

func NewLinuxKernelTelemetrySource(probe *LinuxEBPFProbe) *LinuxKernelTelemetrySource { return &LinuxKernelTelemetrySource{Probe:probe} }

func readCurrentRSSBytes(pid uint32) (uint64,error) {
	data,err:=os.ReadFile(filepath.Join("/proc",strconv.FormatUint(uint64(pid),10),"statm")); if err!=nil{return 0,err}
	f:=strings.Fields(string(data)); if len(f)<2{return 0,errors.New("invalid /proc statm")}
	r,err:=strconv.ParseUint(f[1],10,64); if err!=nil{return 0,err}; return r*uint64(os.Getpagesize()),nil
}
func readMemoryTotalBytes()(uint64,error){
	data,err:=os.ReadFile("/proc/meminfo"); if err!=nil{return 0,err}
	for _,line:=range strings.Split(string(data),"\n"){ f:=strings.Fields(line); if len(f)>=2&&f[0]=="MemTotal:"{k,err:=strconv.ParseUint(f[1],10,64);if err!=nil{return 0,err};return k*1024,nil} }
	return 0,errors.New("MemTotal missing from /proc/meminfo")
}
func readGoCPUSeconds() float64 {
	s:=[]metrics.Sample{{Name:"/cpu/classes/user:cpu-seconds"},{Name:"/cpu/classes/gc/total:cpu-seconds"}}; metrics.Read(s)
	return s[0].Value.Float64()+s[1].Value.Float64()
}
func (s *LinuxKernelTelemetrySource) Snapshot(ctx context.Context,pid uint32)(KernelTelemetrySnapshot,error){
	if s==nil||s.Probe==nil{return KernelTelemetrySnapshot{},errors.New("Linux kernel telemetry source requires a live eBPF probe")}
	if ctx==nil{ctx=context.Background()}; if err:=ctx.Err();err!=nil{return KernelTelemetrySnapshot{},err}
	if pid==0{pid=uint32(os.Getpid())}
	rss,err:=readCurrentRSSBytes(pid);if err!=nil{return KernelTelemetrySnapshot{},fmt.Errorf("read process RSS: %w",err)}
	total,err:=readMemoryTotalBytes();if err!=nil{return KernelTelemetrySnapshot{},fmt.Errorf("read host memory total: %w",err)}
	var ms runtime.MemStats; runtime.ReadMemStats(&ms)
	now:=time.Now().UTC(); cpu:=readGoCPUSeconds()
	s.mu.Lock(); pct:=int64(0)
	if !s.lastWall.IsZero(){ elapsed:=now.Sub(s.lastWall).Seconds(); delta:=cpu-s.lastCPU; if elapsed>0&&delta>=0{cap:=elapsed*float64(max(1,runtime.GOMAXPROCS(0))); pct=int64((delta/cap)*100);if pct<0{pct=0};if pct>100{pct=100} } }
	s.lastCPU=cpu;s.lastWall=now;s.mu.Unlock()
	counts:=s.Probe.Snapshot(pid)
	snapshot:=KernelTelemetrySnapshot{PID:pid,ObservedAtUnixNanos:now.UnixNano(),CPUPercent:pct,RSSBytes:rss,TotalAllocBytes:ms.TotalAlloc,MemoryTotalBytes:total,SchedulerEvents:counts.SchedulerEvents,AllocationEvents:counts.AllocationEvents}
	return snapshot,snapshot.Validate()
}

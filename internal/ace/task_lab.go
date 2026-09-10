package ace

import (
    "errors"
    "strconv"
)

type TaskFamily interface { Name() string; Generate(int, bool) (Task, []ProgramTestCase, CapabilityOracle, error) }
type LatentTaskLab struct{ Families []TaskFamily }
func (l LatentTaskLab) GenerateDiscovery(seed int)(Task,[]ProgramTestCase,CapabilityOracle,error){if len(l.Families)==0{return Task{},nil,nil,errors.New("no task families")};return l.Families[seed%len(l.Families)].Generate(seed,false)}
func (l LatentTaskLab) GenerateHidden(seed int)(Task,[]ProgramTestCase,CapabilityOracle,error){if len(l.Families)==0{return Task{},nil,nil,errors.New("no task families")};return l.Families[seed%len(l.Families)].Generate(seed,true)}
type functionOracle func(map[string]string)(map[string]string,error)
func(f functionOracle)Evaluate(_ Task,in map[string]string)(map[string]string,error){return f(in)}

type AffineFamily struct{}
func(AffineFamily)Name()string{return "affine"}
func(AffineFamily)Generate(seed int,hidden bool)(Task,[]ProgramTestCase,CapabilityOracle,error){shift:=seed%5+2;cases:=[]ProgramTestCase{{Input:map[string]string{"x":"-2"},Expected:map[string]string{"y":strconv.Itoa(-2+shift)}},{Input:map[string]string{"x":"3"},Expected:map[string]string{"y":strconv.Itoa(3+shift)}}};oracle:=functionOracle(func(in map[string]string)(map[string]string,error){n,e:=strconv.Atoi(in["x"]);if e!=nil{return nil,e};return map[string]string{"y":strconv.Itoa(n+shift)},nil});return Task{ID:Hash([]any{"affine",seed,hidden}),Goal:"y equals x plus latent shift",Requirements:[]string{"x"},Structure:[]string{"scalar","affine"},Novel:hidden,Budget:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20}},cases,oracle,nil}

type ThresholdFamily struct{}
func(ThresholdFamily)Name()string{return "threshold"}
func(ThresholdFamily)Generate(seed int,hidden bool)(Task,[]ProgramTestCase,CapabilityOracle,error){threshold:=seed%5+1;cases:=[]ProgramTestCase{{Input:map[string]string{"x":"0"},Expected:map[string]string{"y":"0"}},{Input:map[string]string{"x":strconv.Itoa(threshold+1)},Expected:map[string]string{"y":"1"}}};oracle:=functionOracle(func(in map[string]string)(map[string]string,error){n,e:=strconv.Atoi(in["x"]);if e!=nil{return nil,e};if n>threshold{return map[string]string{"y":"1"},nil};return map[string]string{"y":"0"},nil});return Task{ID:Hash([]any{"threshold",seed,hidden}),Goal:"y indicates whether x exceeds latent threshold",Requirements:[]string{"x"},Structure:[]string{"scalar","conditional","threshold"},Novel:hidden,Budget:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20}},cases,oracle,nil}

type DeepCompositionFamily struct{}
func(DeepCompositionFamily)Name()string{return "deep-composition"}
func(DeepCompositionFamily)Generate(seed int,hidden bool)(Task,[]ProgramTestCase,CapabilityOracle,error){shift:=seed%3+1;cases:=[]ProgramTestCase{{Input:map[string]string{"x":"1"},Expected:map[string]string{"y":strconv.Itoa((1+shift)*2+1)}},{Input:map[string]string{"x":"3"},Expected:map[string]string{"y":strconv.Itoa((3+shift)*2+1)}}};oracle:=functionOracle(func(in map[string]string)(map[string]string,error){n,e:=strconv.Atoi(in["x"]);if e!=nil{return nil,e};return map[string]string{"y":strconv.Itoa((n+shift)*2+1)},nil});return Task{ID:Hash([]any{"deep",seed,hidden}),Goal:"y is shifted, doubled, then incremented",Requirements:[]string{"x"},Structure:[]string{"scalar","composition","depth-3"},Novel:hidden,Budget:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20}},cases,oracle,nil}

type ReplicatedDeepFamily struct{}
func(ReplicatedDeepFamily)Name()string{return "replicated-deep-composition"}
func(ReplicatedDeepFamily)Generate(seed int,hidden bool)(Task,[]ProgramTestCase,CapabilityOracle,error){x:=seed%17-8;cases:=[]ProgramTestCase{{Input:map[string]string{"x":strconv.Itoa(x)},Expected:map[string]string{"y":strconv.Itoa((x+2)*2+1)}},{Input:map[string]string{"x":strconv.Itoa(x+5)},Expected:map[string]string{"y":strconv.Itoa((x+7)*2+1)}}};oracle:=functionOracle(func(in map[string]string)(map[string]string,error){n,e:=strconv.Atoi(in["x"]);if e!=nil{return nil,e};return map[string]string{"y":strconv.Itoa((n+2)*2+1)},nil});return Task{ID:Hash([]any{"replicated",seed,hidden}),Goal:"deep composition",Requirements:[]string{"x"},Structure:[]string{"scalar","composition","depth-3"},Novel:hidden,Budget:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20}},cases,oracle,nil}

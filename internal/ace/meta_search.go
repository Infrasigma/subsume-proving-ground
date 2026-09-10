package ace

import (
    "errors"
    "fmt"
    "strconv"
)

type MetaAcquisitionResult struct { Program UniversalProgram; Candidates int; Method string; Verified bool; Diagnosis string }

func ParameterizedMechanismSearch(cases []ProgramTestCase)(MetaAcquisitionResult,error){
    if len(cases)<2{return MetaAcquisitionResult{},errors.New("need behavioral evidence")}
    in,out:="", ""
    for k:=range cases[0].Input { in=k; break }
    for k:=range cases[0].Expected { out=k; break }
    if in==""||out=="" { return MetaAcquisitionResult{},errors.New("missing input/output") }
    constants:=map[int]bool{-10:true,-5:true,-4:true,-3:true,-2:true,-1:true,0:true,1:true,2:true,3:true,4:true,5:true,10:true}
    for _,tc:=range cases { a,ea:=strconv.Atoi(tc.Input[in]); b,eb:=strconv.Atoi(tc.Expected[out]); if ea==nil&&eb==nil { constants[b-a]=true; if a!=0&&b%a==0 { constants[b/a]=true } } }
    candidates:=[]UniversalProgram{}
    for c:=range constants {
        add:=UniversalProgram{Statements:[]UStmt{{Kind:"assign",Target:out,Expr:&UExpr{Kind:"add",Left:&UExpr{Kind:"var",Value:in},Right:&UExpr{Kind:"const",Value:strconv.Itoa(c)}}}}}
        mul:=UniversalProgram{Statements:[]UStmt{{Kind:"assign",Target:out,Expr:&UExpr{Kind:"mul",Left:&UExpr{Kind:"var",Value:in},Right:&UExpr{Kind:"const",Value:strconv.Itoa(c)}}}}}
        candidates=append(candidates,add,mul)
    }
    for i,p:=range candidates { if programFits(p,cases) { return MetaAcquisitionResult{Program:p,Candidates:i+1,Method:"parameterized-mechanism-search",Verified:true,Diagnosis:fmt.Sprintf("generic candidate %d verified",i+1)},nil } }
    return MetaAcquisitionResult{Candidates:len(candidates),Method:"parameterized-mechanism-search",Diagnosis:"generic arithmetic search exhausted"},errors.New("parameterized search exhausted")
}

func composePrograms(first,second UniversalProgram,input string)(UniversalProgram,error){
    if len(first.Statements)!=1||len(second.Statements)!=1{return UniversalProgram{},errors.New("single-assignment programs required")}
    a,b:=first.Statements[0].Expr,second.Statements[0].Expr
    if a==nil||b==nil{return UniversalProgram{},errors.New("missing expression")}
    var sub func(*UExpr)*UExpr
    sub=func(e *UExpr)*UExpr{if e==nil{return nil};if e.Kind=="var"&&e.Value==input{return cloneExpr(*a)};x:=cloneExpr(*e);if e.Left!=nil{x.Left=sub(e.Left)};if e.Right!=nil{x.Right=sub(e.Right)};return x}
    return UniversalProgram{Statements:[]UStmt{{Kind:"assign",Target:second.Statements[0].Target,Expr:sub(b)}}},nil
}

func programFitsJSON(artifact string,cases []ProgramTestCase)bool{var p UniversalProgram;if err:=unmarshalJSON([]byte(artifact),&p);err!=nil{return false};return programFits(p,cases)}

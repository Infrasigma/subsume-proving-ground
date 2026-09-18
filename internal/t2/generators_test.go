package t2

import "testing"

func TestDefaultNoveltyPredicate(t *testing.T){p:=DefaultGeneratorPlan(1);if p.Novelty.IsNovel(StructuralVector{ASTDepth:2,GraphNodeCount:4,CycleRank:1,DependencyPathLength:3}){t.Fatal("false positive novelty")};if !p.Novelty.IsNovel(StructuralVector{ASTDepth:3}){t.Fatal("AST novelty missing")};if !p.Novelty.IsNovel(StructuralVector{GraphNodeCount:6}){t.Fatal("graph-node novelty missing")};if !p.Novelty.IsNovel(StructuralVector{CycleRank:2}){t.Fatal("cycle novelty missing")};if !p.Novelty.IsNovel(StructuralVector{DependencyPathLength:5}){t.Fatal("path novelty missing")}}

func TestGenerateTaskPool(t *testing.T){p:=DefaultGeneratorPlan(99);tasks,err:=GenerateTaskPool(p);if err!=nil{t.Fatal(err)};if len(tasks)!=len(p.Generators)*(p.NonNovelPerFamily+p.NovelPerFamily){t.Fatalf("unexpected task count %d",len(tasks))};for _,x:=range tasks{if p.Novelty.IsNovel(x.Public.Structure)!=x.Public.Novel{t.Fatalf("novelty mismatch %s",x.Public.ID)}}}

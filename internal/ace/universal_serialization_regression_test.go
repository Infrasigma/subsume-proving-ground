package ace

import "testing"

func TestUniversalProgramSerializationPreservesBothBranchesAndBinaryOperands(t *testing.T) {
	p := UniversalProgram{Statements: []UStmt{{Kind: "if", Cond: &UExpr{Kind: "lt", Left: &UExpr{Kind: "var", Value: "x"}, Right: &UExpr{Kind: "const", Value: "0"}}, Then: []UStmt{{Kind: "assign", Target: "y", Expr: &UExpr{Kind: "sub", Left: &UExpr{Kind: "const", Value: "0"}, Right: &UExpr{Kind: "var", Value: "x"}}}}, Else: []UStmt{{Kind: "assign", Target: "y", Expr: &UExpr{Kind: "add", Left: &UExpr{Kind: "var", Value: "x"}, Right: &UExpr{Kind: "const", Value: "1"}}}}}}}
	cases := []ProgramTestCase{{Input: map[string]string{"x": "-3"}, Expected: map[string]string{"y": "3"}}, {Input: map[string]string{"x": "2"}, Expected: map[string]string{"y": "3"}}}
	if !serializedProgramFits(p, cases) { t.Fatal("serialization changed executable branch semantics") }
}

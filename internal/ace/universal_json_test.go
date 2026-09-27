package ace

import (
    "encoding/json"
    "testing"
)

func TestUniversalProgramJSONDistinguishesBinaryAndBranchSides(t *testing.T) {
    stmt := UStmt{
        Kind: "if",
        Cond: &UExpr{
            Kind: "eq",
            Left: &UExpr{Kind: "var", Value: "a"},
            Right: &UExpr{Kind: "const", Value: "0"},
        },
        Then: []UStmt{{
            Kind: "assign",
            Target: "out",
            Expr: &UExpr{
                Kind: "add",
                Left: &UExpr{Kind: "var", Value: "a"},
                Right: &UExpr{Kind: "const", Value: "1"},
            },
        }},
        Else: []UStmt{{
            Kind: "assign",
            Target: "out",
            Expr: &UExpr{Kind: "const", Value: "-1"},
        }},
    }

    raw, err := json.Marshal(stmt)
    if err != nil {
        t.Fatalf("marshal: %v", err)
    }

    var decoded map[string]json.RawMessage
    if err := json.Unmarshal(raw, &decoded); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }

    for _, key := range []string{"then", "else"} {
        if _, ok := decoded[key]; !ok {
            t.Fatalf("missing branch key %q in %s", key, raw)
        }
    }

    var cond map[string]json.RawMessage
    if err := json.Unmarshal(decoded["cond"], &cond); err != nil {
        t.Fatalf("decode condition: %v", err)
    }
    for _, key := range []string{"left", "right"} {
        if _, ok := cond[key]; !ok {
            t.Fatalf("missing operand key %q in %s", key, decoded["cond"])
        }
    }
}

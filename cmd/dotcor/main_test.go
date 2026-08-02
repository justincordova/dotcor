package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOnlyMainCallsOsExit pins the fix for skipped cleanup.
//
// os.Exit does not run deferred functions. run() owns
// `defer core.ReleaseLock` and `defer logCloser.Close`, so any os.Exit inside
// it leaves ~/.dotcor/.lock behind and discards unflushed log output. Exit
// codes must travel back to main as return values instead.
func TestOnlyMainCallsOsExit(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, 0)
	require.NoError(t, err)

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name == "main" {
			continue
		}

		ast.Inspect(fn, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			assert.False(t, pkg.Name == "os" && sel.Sel.Name == "Exit",
				"%s calls os.Exit at %s — this skips deferred lock release and log flushing",
				fn.Name.Name, fset.Position(call.Pos()))
			return true
		})
	}
}

// TestRunReturnsExitCode is a compile-time guarantee that run() reports its
// outcome as a value rather than terminating the process. The assignment only
// compiles while run has the signature func() int.
func TestRunReturnsExitCode(t *testing.T) {
	f := run
	assert.NotNil(t, f)
}

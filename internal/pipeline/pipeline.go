package pipeline

import (
	"encoding/json"
	"fmt"
	"gocomp/internal/parser"
	"gocomp/internal/passes"

	"github.com/antlr4-go/antlr/v4"
	"github.com/llir/llvm/ir"
)

func ProcessTree(ctx parser.ISourceFileContext) (*ir.Module, error) {
	pass1 := passes.NewPackageListener()
	antlr.ParseTreeWalkerDefault.Walk(pass1, ctx)
	result := pass1.PackageData()

	ast1, _ := json.MarshalIndent(result, "    ", "  ")
	fmt.Printf("package data:\n%s\n", ast1)

	pass2 := passes.NewCodeGenVisitor(result)
	return pass2.VisitSourceFile(ctx)
}

package pipeline

import (
	"encoding/json"
	"fmt"
	"gocomp/internal/parser"
	"gocomp/internal/passes"

	"github.com/antlr4-go/antlr/v4"
)

func ProcessTree(ctx parser.ISourceFileContext) interface{} {
	pass1 := passes.NewPackageListener()
	antlr.ParseTreeWalkerDefault.Walk(pass1, ctx)
	result := pass1.PackageData()

	ast1, _ := json.MarshalIndent(result, "    ", "  ")
	fmt.Printf("package data:\n%s\n", ast1)

	pass2 := passes.NewCodeGenVisitor(result)
	return ctx.Accept(pass2)
}

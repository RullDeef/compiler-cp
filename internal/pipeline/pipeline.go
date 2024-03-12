package pipeline

import (
	"encoding/json"
	"fmt"
	"gocomp/internal/parser"
	"gocomp/internal/passes"
)

func ProcessTree(ctx parser.ISourceFileContext) interface{} {
	pass1 := passes.NewPackageVisitor()
	result := ctx.Accept(pass1).(*passes.PackageData)

	ast1, _ := json.MarshalIndent(result, "    ", "  ")
	fmt.Printf("package data:\n%s\n", ast1)

	pass2 := passes.NewCodeGenVisitor(result)
	return ctx.Accept(pass2)
}

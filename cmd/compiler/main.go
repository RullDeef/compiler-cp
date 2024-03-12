package main

import (
	"encoding/json"
	"fmt"
	"gocomp/internal/parser"
	"gocomp/internal/pipeline"
	"os"

	"github.com/antlr4-go/antlr/v4"
	"github.com/llir/llvm/ir"
)

func main() {
	data := `package main

func foo(a int) int {
	return 10 + (2 - 3) / 2
}

func main() {
	return
}`
	lexer := parser.NewGoLexer(antlr.NewInputStream(data))
	tokenStream := antlr.NewCommonTokenStream(lexer, antlr.LexerDefaultTokenChannel)
	parser := parser.NewGoParser(tokenStream)

	sourceFileContext := parser.SourceFile()
	result := pipeline.ProcessTree(sourceFileContext).(*ir.Module)

	res, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("result:\n%s\n", res)

	result.WriteTo(os.Stdout)
}

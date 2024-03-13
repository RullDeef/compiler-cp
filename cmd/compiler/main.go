package main

import (
	"encoding/json"
	"fmt"
	"gocomp/internal/parser"
	"gocomp/internal/pipeline"
	"os"

	"github.com/antlr4-go/antlr/v4"
)

func main() {
	data := `package main

func retA(a, b, c int) int {
	return a + b + c
}

func main() {
	return
}`
	lexer := parser.NewGoLexer(antlr.NewInputStream(data))
	tokenStream := antlr.NewCommonTokenStream(lexer, antlr.LexerDefaultTokenChannel)
	parser := parser.NewGoParser(tokenStream)

	sourceFileContext := parser.SourceFile()
	module, err := pipeline.ProcessTree(sourceFileContext)
	if err != nil {
		panic(err)
	}

	res, _ := json.MarshalIndent(module, "", "  ")
	fmt.Printf("result:\n%s\n", res)

	module.WriteTo(os.Stdout)
}

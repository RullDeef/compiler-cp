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

func avg(a, b float64) float64 {
	return (a + b) / 2.0
}

func main() {
	a := 190.0
	b := 20.0
	c := avg(a, b + 5.0)
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

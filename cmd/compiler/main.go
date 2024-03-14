package main

import (
	"gocomp/internal/parser"
	"gocomp/internal/pipeline"
	"os"

	"github.com/antlr4-go/antlr/v4"
)

func main() {
	data := `package main

// printf(format i8*, ...)

func avg(a, b float64) float64 {
	return (a + b) / 2.0
}

func main() int {
	printf("avg of 2 and 3 is %f\n", avg(2.0, 3.0))
	return 0
}`
	lexer := parser.NewGoLexer(antlr.NewInputStream(data))
	tokenStream := antlr.NewCommonTokenStream(lexer, antlr.LexerDefaultTokenChannel)
	parser := parser.NewGoParser(tokenStream)

	sourceFileContext := parser.SourceFile()
	module, err := pipeline.ProcessTree(sourceFileContext)
	if err != nil {
		panic(err)
	}

	module.WriteTo(os.Stdout)
}

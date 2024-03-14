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

func min3(a, b, c float64) float64 {
	if a < b {
		if a < c {
			return a
		} else {
			return c
		}
	} else if c > b {
		return b
	} else {
		return c
	}
}

func main() int {
	s := "min of 3, 4, 5 is %f\n"
	printf(s, min3(3.0, 4.0, 5.0))
	printf(s, min3(3.0, 5.0, 4.0))
	printf(s, min3(4.0, 3.0, 5.0))
	printf(s, min3(4.0, 5.0, 3.0))
	printf(s, min3(5.0, 4.0, 3.0))
	printf(s, min3(5.0, 3.0, 4.0))
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

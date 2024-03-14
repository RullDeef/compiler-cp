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

func fib(n int) int {
	if n <= 1 {
		return n
	} else {
		return fib(n-1) + fib(n-2)
	}
}

func fact(n int) int {
	if n <= 1 {
		return 1
	} else {
		return n * fact(n-1)
	}
}

func main() int {
	printf("6th Fibonacci number is %d\n", fib(6))
	printf("6! = %d\n", fact(6))
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

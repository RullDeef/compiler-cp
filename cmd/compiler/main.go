package main

import (
	"gocomp/internal/parser"
	"gocomp/internal/pipeline"
	"io"
	"os"

	"github.com/antlr4-go/antlr/v4"
)

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	lexer := parser.NewGoLexer(antlr.NewInputStream(string(data)))
	tokenStream := antlr.NewCommonTokenStream(lexer, antlr.LexerDefaultTokenChannel)
	parser := parser.NewGoParser(tokenStream)

	sourceFileContext := parser.SourceFile()
	module, err := pipeline.ProcessTree(sourceFileContext)
	if err != nil {
		panic(err)
	}

	module.WriteTo(os.Stdout)
}

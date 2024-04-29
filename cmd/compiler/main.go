package main

import (
	"gocomp/internal/parser"
	"gocomp/internal/pipeline"
	"io"
	"os"

	"github.com/antlr4-go/antlr/v4"
)

func main() {
	var data []byte
	if len(os.Args) > 1 {
		var err error
		data, err = os.ReadFile(os.Args[1])
		if err != nil {
			panic(err)
		}
	} else {
		var err error
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			panic(err)
		}
	}

	lexer := parser.NewGoLexer(antlr.NewInputStream(string(data)))
	tokenStream := antlr.NewCommonTokenStream(lexer, antlr.LexerDefaultTokenChannel)
	parser := parser.NewGoParser(tokenStream)

	sourceFileContext := parser.SourceFile()
	module, err := pipeline.ProcessTree(sourceFileContext)
	if err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(-1)
	}

	module.WriteTo(os.Stdout)
}

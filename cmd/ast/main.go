package main

import (
	"fmt"
	"gocomp/internal/parser"
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

	fmt.Println(antlr.TreesStringTree(sourceFileContext, parser.RuleNames, parser))
}

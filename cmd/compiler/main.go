package main

import (
	"encoding/json"
	"fmt"
	"gocomp/internal/parser"
	"gocomp/internal/passes"

	"github.com/antlr4-go/antlr/v4"
)

func main() {
	data := `package main

import "fmt"
import "os"

var a int = 10
func foo() {}
func pepe(a, b int, c string) float64 {}

func (OmegaStruct) New(args ...int) *OmegaStruct {}`
	lexer := parser.NewGoLexer(antlr.NewInputStream(data))
	tokenStream := antlr.NewCommonTokenStream(lexer, antlr.LexerDefaultTokenChannel)
	parser := parser.NewGoParser(tokenStream)

	sourceFileContext := parser.SourceFile()

	pass1 := passes.NewPackageVisitor()
	result := sourceFileContext.Accept(pass1)

	ast1, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("parse result:\n%s\n", ast1)
}

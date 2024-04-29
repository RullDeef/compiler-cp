package utils

import (
	"fmt"

	"github.com/antlr4-go/antlr/v4"
)

func MakeError(format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	// panic(err)
	return err
}

func MakeErrorTrace(ctx antlr.ParserRuleContext, prevErr error, format string, args ...any) error {
	tok := ctx.GetStart()
	errMsg := fmt.Sprintf("<input>:%d:%d: %s", tok.GetLine(), tok.GetColumn(), fmt.Sprintf(format, args...))
	if prevErr == nil {
		return fmt.Errorf("%s", errMsg)
	}
	return fmt.Errorf("%s\n%w", errMsg, prevErr)
}

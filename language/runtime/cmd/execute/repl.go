package execute

import (
	"fmt"
	"os"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/logrusorgru/aurora"

	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/language/runtime/interpreter"
)

const replFilename = "REPL"

func RunREPL() {
	printWelcome()

	lineNumber := 1
	lineIsContinuation := false
	code := ""

	repl, err := runtime.NewREPL(
		func(err error) {
			// TODO: handle imports
			PrettyPrintError(err, replFilename, map[string]string{replFilename: code})
		},
		func(value interpreter.Value) {
			if _, isVoid := value.(*interpreter.VoidValue); isVoid || value == nil {
				return
			}

			println(colorizeResult(value))
		},
	)

	if err != nil {
		panic(err)
	}

	executor := func(line string) {
		defer func() {
			lineNumber += 1
		}()

		code += line + "\n"

		inputIsComplete := repl.Accept(code)
		if !inputIsComplete {
			lineIsContinuation = true
			return
		}

		lineIsContinuation = false
		code = ""
	}

	suggest := func(document prompt.Document) []prompt.Suggest {
		return nil
	}

	changeLivePrefix := func() (string, bool) {
		separator := '>'
		if lineIsContinuation {
			separator = '.'
		}

		return fmt.Sprintf("%d%c ", lineNumber, separator), true
	}

	options := []prompt.Option{
		prompt.OptionLivePrefix(changeLivePrefix),
	}
	prompt.New(executor, suggest, options...).Run()
}


const assistanceMessage = `Type '.help' for assistance.`

func printWelcome() {
	fmt.Printf("Welcome to Cadence!\n%s\n\n", assistanceMessage)
}

func colorizeResult(value interpreter.Value) string {
	return aurora.Colorize(fmt.Sprint(value), aurora.YellowFg|aurora.BrightFg).String()
}

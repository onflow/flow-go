package vscode

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cli"
)

const cadenceExt = "cadence.vsix"

var Cmd = &cobra.Command{
	Use:   "install-vscode-extension",
	Short: "Install the Cadence Visual Studio Code extension",
	Run: func(cmd *cobra.Command, args []string) {
		ext, _ := Asset(cadenceExt)

		// create temporary directory
		dir, err := ioutil.TempDir("", "vscode-cadence")
		if err != nil {
			cli.Exit(1, err.Error())
		}

		// delete temporary directory
		defer os.RemoveAll(dir)

		tmpCadenceExt := fmt.Sprintf("%s/%s", dir, cadenceExt)

		err = ioutil.WriteFile(tmpCadenceExt, ext, 0644)
		if err != nil {
			cli.Exit(1, err.Error())
		}

		// run vscode command to install extension from temporary directory
		c := exec.Command("code", "--install-extension", tmpCadenceExt)
		err = c.Run()
		if err != nil {
			cli.Exit(1, err.Error())
		}

		fmt.Println("Installed the Cadence Visual Studio Code extension")
	},
}

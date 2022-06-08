package cmd

import (
	"github.com/spf13/cobra"
)

// MarkFlagRequired marks a flag added to a cobra command as required. Panics
// if the flag has not been added to the cobra command (indicates misconfiguration
// or typo).
func MarkFlagRequired(command *cobra.Command, flagName string) {
	err := command.MarkFlagRequired(flagName)
	if err != nil {
		panic("marked unknown flag as required: " + err.Error())
	}
}

package client

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func Command(log *logrus.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "client",
		Short: "Bamboo Client operations",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("client called")
		},
	}
}

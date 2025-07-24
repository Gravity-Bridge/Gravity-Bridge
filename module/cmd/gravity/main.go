package main

import (
	"fmt"
	"os"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/cmd/gravity/cmd"
	_ "github.com/Gravity-Bridge/Gravity-Bridge/module/config"
)

func main() {
	rootCmd, _ := cmd.NewRootCmd()
	if err := cmd.Execute(rootCmd); err != nil {
		fmt.Fprintln(rootCmd.OutOrStderr(), err)
		os.Exit(1)
	}
}

package main

import (
	"os"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/cmd/gravity/cmd"
	_ "github.com/Gravity-Bridge/Gravity-Bridge/module/config"
	"github.com/cosmos/cosmos-sdk/server"
)

func main() {
	rootCmd, _ := cmd.NewRootCmd()
	if err := cmd.Execute(rootCmd); err != nil {
		switch e := err.(type) {
		case server.ErrorCode:
			os.Exit(e.Code)
		default:
			os.Exit(1)
		}
	}
}

package commands

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "gogeta",
	Short: "gogeta is a distributed load testing framework",
	Long: `A lightweight and fast load testing framework
			inspired by https://github.com/tsenart/vegeta.
			Supports scripting using yaml/json`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

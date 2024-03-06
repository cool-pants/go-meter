package commands

import (
	"fmt"
	gogeta "github.com/cool-pants/gogeta/utils"
	"github.com/spf13/cobra"
	"time"
)

func init() {
	rootCmd.AddCommand(AttackCommand())
}

func AttackCommand() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     "attack",
		Short:   "attack the targets",
		Example: "cat plan.yaml | gogeta attack -r 10/s",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Hello, World!")
		},
	}

	opts := &attackOpts{
		laddr: localAddr{&gogeta.DefaultLocalAddr},
		rate:  gogeta.Rate{Freq: 50, Per: time.Second},
	}

	cmd.Flags().StringVar(&opts.name, "name", "", "Attack name")
	cmd.Flags().StringVarP(&opts.target, "target", "t", "stdin", "Targets file")
	cmd.Flags().StringVarP(&opts.format, "format", "f", "http", "Target Formats http/yaml/json")
	cmd.Flags().Var(&rateFlag{&opts.rate}, "rate", "Rate of the requests to be sent")
	cmd.Flags().Uint64Var(&opts.workers, "workers", gogeta.DefaultWorkers, "Number of Virtual Users to be used")
	cmd.Flags().Uint64Var(&opts.maxWorkers, "maxWorkers", gogeta.DefaultMaxWorkers, "Max Number of Virtual Users to be used")
	cmd.Flags().Var(&rateFlag{&opts.workerRamp}, "workersRamp", "Ramp rate of workers")
	cmd.Flags().IntVar(&opts.connections, "connections", gogeta.DefaultConnections, "Max open idle connections per target host")
	cmd.Flags().IntVar(&opts.maxConnections, "maxConnections", gogeta.DefaultMaxConnections, "Max connections per target host")
	cmd.Flags().Var(&opts.laddr, "laddr", "Local IP address")
	cmd.Flags().BoolVar(&opts.keepalive, "keepalive", true, "Use persistent connections")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "stdout", "Output File")

	return cmd
}

type attackOpts struct {
	name           string
	target         string
	format         string
	rate           gogeta.Rate
	workers        uint64
	maxWorkers     uint64
	workerRamp     gogeta.Rate
	connections    int
	maxConnections int
	laddr          localAddr
	keepalive      bool
	output         string
}

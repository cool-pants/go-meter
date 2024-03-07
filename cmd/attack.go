package commands

import (
	"fmt"
	gogeta "github.com/cool-pants/gogeta/utils"
	"github.com/spf13/cobra"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func init() {
	rootCmd.AddCommand(AttackCommand())
}

var opts = &attackOpts{
	laddr: localAddr{&gogeta.DefaultLocalAddr},
	rate:  gogeta.Rate{Freq: 50, Per: time.Second},
}

func AttackCommand() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     "attack",
		Short:   "attack the targets",
		Example: "cat plan.yaml | gogeta attack -r 10/s",
		RunE:    attack,
	}

	cmd.Flags().StringVar(&opts.name, "name", "", "Attack name")
	cmd.Flags().StringVarP(&opts.target, "target", "t", "stdin", "Targets file")
	cmd.Flags().StringVarP(&opts.format, "format", "f", "http", "Target Formats http/yaml/json")
	cmd.Flags().VarP(&rateFlag{&opts.rate}, "rate", "r", "Rate of the requests to be sent")
	cmd.Flags().Uint64Var(&opts.workers, "workers", gogeta.DefaultWorkers, "Number of Virtual Users to be used")
	cmd.Flags().Uint64Var(&opts.maxWorkers, "maxWorkers", gogeta.DefaultMaxWorkers, "Max Number of Virtual Users to be used")
	cmd.Flags().Var(&rateFlag{&opts.workerRamp}, "workersRamp", "Ramp rate of workers")
	cmd.Flags().IntVar(&opts.connections, "connections", gogeta.DefaultConnections, "Max open idle connections per target host")
	cmd.Flags().IntVar(&opts.maxConnections, "maxConnections", gogeta.DefaultMaxConnections, "Max connections per target host")
	cmd.Flags().Var(&opts.laddr, "laddr", "Local IP address")
	cmd.Flags().BoolVar(&opts.keepalive, "keepalive", true, "Use persistent connections")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "stdout", "Output File")
	cmd.Flags().DurationVar(&opts.duration, "duration", 0, "Duration of the test [0 = forever]")

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
	duration       time.Duration
}

func handleErrors(err error, msg string) {
	if err != nil {
		fmt.Println(msg)
		os.Exit(1)
	}
}

func attack(cmd *cobra.Command, args []string) error {
	var reader = cmd.InOrStdin()
	if opts.target != "stdin" {
		file, err := os.Open(opts.target)
		handleErrors(err, fmt.Sprintf("Error Opening target file: %v", err))
		reader = file
	}
	targets := gogeta.ProcessReader(reader)

	tr := gogeta.NewStaticTargeter(targets...)

	net.DefaultResolver.PreferGo = true

	out, err := file(opts.output, true)
	if err != nil {
		handleErrors(err, fmt.Sprintf("error opening %s: %s", opts.output, err))
	}
	defer out.Close()

	atk := gogeta.NewAttacker(
		gogeta.LocalAddr(*opts.laddr.IPAddr),
		gogeta.Workers(opts.workers),
		gogeta.MaxWorkers(opts.maxWorkers),
		gogeta.KeepAlive(opts.keepalive),
		gogeta.Connections(opts.connections),
		gogeta.MaxConnections(opts.maxConnections),
	)

	res := atk.Attack(tr, opts.rate, opts.duration, opts.name)
	enc := gogeta.NewEncoder(out)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	return processAttack(atk, res, enc, sig)

}

func processAttack(
	atk *gogeta.Attacker,
	res <-chan *gogeta.Result,
	enc gogeta.Encoder,
	sig <-chan os.Signal,
) error {
	for {
		select {
		case <-sig:
			if stopSent := atk.Stop(); !stopSent {
				// Exit immediately on second signal.
				return nil
			}
		case r, ok := <-res:
			if !ok {
				return nil
			}

			if err := enc.Encode(r); err != nil {
				return err
			}
		}
	}
}

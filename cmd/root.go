package cmd

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sort"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	c "github.com/secretanalytics/go-scrt-events/config"
	"github.com/secretanalytics/go-scrt-events/pkg/node"
	"github.com/secretanalytics/go-scrt-events/pkg/types"
	"github.com/secretanalytics/go-scrt-events/pkg/db"
)

var (
	// Used for flags.
	cfgFile string
	v       string
	rootCmd = &cobra.Command{
		Use:   "go-scrt-events",
		Short: "scrt-events quickly bootstraps a postgresql db with the Secret Network blockchain block-results.",
		Long:  `scrt-events quickly bootstraps a postgresql db with the Secret Network blockchain block-results.`,
		Run: func(cmd *cobra.Command, args []string) {
			var configuration c.Configurations
			err := viper.Unmarshal(&configuration)
			if err != nil {
				logrus.Error("Unable to decode into struct, %v", err)
			}
			logrus.Debug("Node host is: ", configuration.Node.Host)
			logrus.Debug("DB conn string is: ", configuration.Database.Conn)
			run(configuration.Database.Conn, configuration.Node.Host, configuration.Node.Path)
		},
	}
)

func emitDone(done chan struct{}, heightsIn chan int, blocksOut chan types.BlockResultDB, chainTip int, wg *sync.WaitGroup) {
	//Checks for existence of block height in slice of heights
	contains := func (checkFor int, inSlice []int) bool {
		for i := range inSlice {
			if i == checkFor {
				return true
			}
		}
		return false
	}
	//If no heights for given chain-id then start at 1, else sort ints and start loop at lowest
	heights := db.GetHeights(dbSession, "secret-2")
	if len(heights) == 0 {
		start := 1
	} else {
		sort.Ints(heights)
		start := heights[0]
	}
	//Loop from dbTip to chainTip, if height i not contained in heights, request for block_results at height i will be made
	for i := start; i <= chainTip; i++ {
		if contains(i, heights) {
			logrus.Debug("Heights contain block ", i)
		} else {
			heightsIn <- i
			logrus.Debug("Requesting height ", i)
		}
	}

	//Loop over received blocks channel, if received block == chaintip, signal done
	for block := range blocksIn {
		outBlock := block.DecodeBlock("secret-2")
		blocksOut <- outBlock
		if outBlock.Height == chainTip {
			close(done)
		}
	}
	wg.Done()
}


func run(dbConn, host, path string) {
	var wg sync.WaitGroup
	heightsIn := make(chan int)
	blocksOut := make(chan types.BlockResultDB)

	chainTip := make(chan int)
	done := make(chan struct{})

	dbSession := db.InitDB(dbConn)
	logrus.Debug("Node host is: ", host)

	wg.Add(1)
	go node.HandleWs(host, path, blocks, &wg)

	latestHeight := <- chainTip
	close(chainTip)

	wg.Add(1)
	go emitDone(done, heightsIn, blocksOut, latestHeight, &wg)

	wg.Add(1)
	go db.InsertBlocks(dbSession, blocks, &wg)

	wg.Wait()
	
	db.GetHeights(dbSession, "secret-2")
}


func ScrtEventsCmd() *cobra.Command {
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := setUpLogs(os.Stdout, v); err != nil {
			return err
		}
		return nil
	}
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.scrt-events/config.json)")
	rootCmd.PersistentFlags().StringVarP(&v, "verbosity", "v", logrus.WarnLevel.String(), "Log level (debug, info, warn, error, fatal, panic")

	return rootCmd
}

func setUpLogs(out io.Writer, level string) error {
	logrus.SetOutput(out)
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	logrus.SetLevel(lvl)
	return nil
}

func initConfig() {
	// Don't forget to read config either from cfgFile or from home directory!
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in home directory with name ".cobra" (without extension).
		viper.AddConfigPath("$HOME/.scrt-events")
		viper.SetConfigName("config.yml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Can't read config:", err)
		os.Exit(1)
	}
}

package cmd

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	c "get-scrt-events-go/config"
	"get-scrt-events-go/pkg/node"
	"get-scrt-events-go/pkg/types"
	"get-scrt-events-go/pkg/db"
)

var (
	// Used for flags.
	cfgFile string
	v       string
	rootCmd = &cobra.Command{
		Use:   "scrt-events",
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

func run(dbConn, host, path string) {
	var wg sync.WaitGroup
	blocks := make(chan types.BlockResultDB)
	
	wg.Add(1)
	go node.HandleWs(host, path, blocks, &wg)

	wg.Add(1)
	go db.InsertBlocks(dbConn, blocks, &wg)

	wg.Wait()
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

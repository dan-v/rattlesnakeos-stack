package cmd

import (
	"fmt"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// TODO: temporarily hardcoded
	cfgFile                   string
	defaultConfigFileBase     = ".rattlesnakeos"
	defaultConfigFileFormat   = "toml"
	defaultConfigFile         = fmt.Sprintf("%v.%v", defaultConfigFileBase, defaultConfigFileFormat)
	defaultConfigFileFullPath string
	configFileFullPath        string
	version                   string
	buildScript 			  string
	buildTemplate 			  string
	lambdaTemplate 			  string
	terraformTemplate         string
)

// Execute the CLI
func Execute(ver, bScript, bTemplate, lTemplate, tTempalte string) {
	version = ver
	buildScript = bScript
	buildTemplate = bTemplate
	lambdaTemplate = lTemplate
	terraformTemplate = tTempalte
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func initConfig() {
	home, err := homedir.Dir()
	if err != nil {
		log.WithError(err).Fatal("couldn't find home dir")
	}
	defaultConfigFileFullPath = fmt.Sprintf("%v/%v", home, defaultConfigFile)

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		configFileFullPath = cfgFile
		if _, err := os.Stat(configFileFullPath); os.IsNotExist(err) {
			log.Infof("Config file %v doesn't exist yet - creating it", configFileFullPath)
			_, err := os.Create(configFileFullPath)
			if err != nil {
				log.Fatalf("Failed to create config file %v", configFileFullPath)
			}
		}
	} else {
		viper.SetConfigName(defaultConfigFileBase)
		viper.SetConfigType(defaultConfigFileFormat)
		viper.AddConfigPath(home)
		configFileFullPath = defaultConfigFileFullPath
	}

	if err := viper.ReadInConfig(); err != nil {
		if viper.ConfigFileUsed() != "" {
			log.Fatalf("Failed to parse config file %v. Error: %v", viper.ConfigFileUsed(), err)
		}
	}
	if viper.ConfigFileUsed() != "" {
		log.Printf("Using config file: %v\n", viper.ConfigFileUsed())
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config-file", "", fmt.Sprintf("config file (default location to look for config is $HOME/%s)", defaultConfigFile))
}

var rootCmd = &cobra.Command{
	Use: "rattlesnakeos-stack",
	Short: "A cross platform tool that provisions all of the AWS infrastructure required to build your own privacy " +
		"focused Android OS on a continuous basis with OTA updates.",
}

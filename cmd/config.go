package cmd

import (
	"errors"
	"fmt"
	"github.com/dan-v/rattlesnakeos-stack/internal/cloudaws"
	"github.com/dan-v/rattlesnakeos-stack/internal/devices"
	"math/rand"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(configCmd)
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Setup config file for rattlesnakeos-stack",
	Run: func(cmd *cobra.Command, args []string) {
		color.Cyan(fmt.Sprintln("Device is the device codename (e.g. sailfish). Supported devices:", supportDevicesOutput))
		validate := func(input string) error {
			if len(input) < 1 {
				return errors.New("Device name is too short")
			}
			if !devices.IsSupportedDevice(input) {
				return errors.New("Invalid device")
			}
			return nil
		}
		devicePrompt := promptui.Prompt{
			Label:    "Device ",
			Default:  viper.GetString("device"),
			Validate: validate,
		}
		result, err := devicePrompt.Run()
		if err != nil {
			log.Fatalf("prompt failed %v\n", err)
		}
		viper.Set("device", result)

		defaultName := fmt.Sprintf("rattlesnakeos-%v-%v", result, randomString(10))
		if viper.GetString("name") != "" {
			defaultName = viper.GetString("name")
		}
		color.Cyan(fmt.Sprintln("Stack name is used as an identifier for all the AWS components that get deployed. THIS NAME MUST BE UNIQUE OR DEPLOYMENT WILL FAIL."))
		validate = func(input string) error {
			if len(input) < 1 {
				return errors.New("Stack name is too short")
			}
			return nil
		}
		namePrompt := promptui.Prompt{
			Label:    "Stack name ",
			Validate: validate,
			Default:  defaultName,
		}
		result, err = namePrompt.Run()
		if err != nil {
			log.Fatalf("prompt failed %v\n", err)
		}
		viper.Set("name", result)

		color.Cyan(fmt.Sprintf("Stack region is the AWS region where you would like to deploy your stack. Valid options: %v\n",
			strings.Join(cloudaws.GetSupportedRegions(), ", ")))
		validate = func(input string) error {
			if len(input) < 1 {
				return errors.New("Stack region is too short")
			}
			if !cloudaws.IsSupportedRegion(input) {
				return errors.New("Invalid region")
			}
			return nil
		}
		regionPrompt := promptui.Prompt{
			Label:    "Stack region ",
			Default:  viper.GetString("region"),
			Validate: validate,
		}
		result, err = regionPrompt.Run()
		if err != nil {
			log.Fatalf("prompt failed %v\n", err)
		}
		viper.Set("region", result)

		color.Cyan(fmt.Sprintln("Email address you would like to send build notifications to."))
		validate = func(input string) error {
			if !strings.Contains(input, "@") {
				return errors.New("Must provide valid email")
			}
			return nil
		}
		emailPrompt := promptui.Prompt{
			Label:    "Email ",
			Validate: validate,
			Default:  viper.GetString("email"),
		}
		result, err = emailPrompt.Run()
		if err != nil {
			log.Fatalf("prompt failed %v\n", err)
		}
		viper.Set("email", result)

		defaultKeypairName := "rattlesnakeos"
		if viper.GetString("ssh-key") != "" {
			defaultKeypairName = viper.GetString("ssh-key")
		}
		color.Cyan(fmt.Sprintln("SSH keypair name is the name of your EC2 keypair that was imported into AWS."))
		validate = func(input string) error {
			if len(input) < 1 {
				return errors.New("SSH keypair name is too short")
			}
			return nil
		}
		keypairPrompt := promptui.Prompt{
			Label:    "SSH Keypair Name ",
			Default:  defaultKeypairName,
			Validate: validate,
		}
		result, err = keypairPrompt.Run()
		if err != nil {
			log.Fatalf("prompt failed %v\n", err)
		}
		viper.Set("ssh-key", result)

		err = viper.WriteConfigAs(configFileFullPath)
		if err != nil {
			log.WithError(err).Fatalf("failed to write config file %s", configFileFullPath)
		}
		log.Infof("rattlesnakeos-stack config file has been written to %v", configFileFullPath)
	},
}

func randomString(strlen int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, strlen)
	for i := range result {
		result[i] = chars[r.Intn(len(chars))]
	}
	return string(result)
}

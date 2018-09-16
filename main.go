package main

import (
	"errors"
	"os"

	"github.com/dan-v/rattlesnakeos-stack/stack"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var version string
var name, region, device, sshKey, maxPrice, skipPrice, schedule, instanceType, instanceRegions string
var remove, preventShutdown, force, skipChromiumBuild bool

var rootCmd = &cobra.Command{
	Use:   "rattlesnakeos-stack",
	Short: "Setup AWS infrastructure to build RattlesnakeOS with OTA updates",
	Args: func(cmd *cobra.Command, args []string) error {
		if device != "marlin" && device != "sailfish" && device != "taimen" && device != "walleye" {
			return errors.New("Must specify either marlin|sailfish|taimen|walleye for device type")
		}
		return nil
	},
	Version: version,
	Run: func(cmd *cobra.Command, args []string) {
		s, err := stack.NewAWSStack(&stack.AWSStackConfig{
			Name:              name,
			Region:            region,
			Device:            device,
			InstanceType:      instanceType,
			InstanceRegions:   instanceRegions,
			SSHKey:            sshKey,
			SkipPrice:         skipPrice,
			MaxPrice:          maxPrice,
			PreventShutdown:   preventShutdown,
			Version:           version,
			Schedule:          schedule,
			Force:             force,
			SkipChromiumBuild: skipChromiumBuild,
		})
		if err != nil {
			log.Fatal(err)
		}

		if !remove {
			if err := s.Apply(); err != nil {
				log.Fatal(err)
			}
		} else {
			if err := s.Destroy(); err != nil {
				log.Fatal(err)
			}
		}
	},
}

func init() {
	rootCmd.Flags().StringVarP(&name, "name", "n", "", "name for stack. note: this must be a valid/unique S3 bucket name.")
	rootCmd.MarkFlagRequired("name")
	rootCmd.Flags().StringVarP(&region, "region", "r", "", "aws region for stack deployment (e.g. us-west-2)")
	rootCmd.MarkFlagRequired("region")
	rootCmd.Flags().StringVarP(&device, "device", "d", "", "device you want to build for: 'marlin' (Pixel XL), 'sailfish' (Pixel), 'taimen' (Pixel 2 XL), 'walleye' (Pixel 2)")
	rootCmd.MarkFlagRequired("device")
	rootCmd.Flags().StringVar(&sshKey, "ssh-key", "", "aws ssh key to add to ec2 spot instances. this is optional but is useful for debugging build issues on the instance.")
	rootCmd.Flags().StringVar(&skipPrice, "skip-price", "0.68", "skip requesting ec2 spot instance if price is above this value to begin with.")
	rootCmd.Flags().StringVar(&maxPrice, "max-price", "1.00", "max ec2 spot instance bid. if this value is too low, you may not obtain an instance or it may terminate during a build.")
	rootCmd.Flags().StringVar(&instanceType, "instance-type", "c5.4xlarge", "EC2 instance type (e.g. c4.4xlarge) to use for the build.")
	rootCmd.Flags().StringVar(&instanceRegions, "instance-regions", "us-west-2,us-west-1,us-east-1,us-east-2", "possible regions to launch spot instance.")
	rootCmd.Flags().StringVar(&schedule, "schedule", "rate(14 days)", "cron expression that defines when to kick off builds. note: if you give invalid expression it will fail to deploy stack. see: https://docs.aws.amazon.com/AmazonCloudWatch/latest/events/ScheduledEvents.html#CronExpressions")
	rootCmd.Flags().BoolVar(&force, "force", false, "build even if there are no changes in available version of AOSP, Chromium, or F-Droid.")
	rootCmd.Flags().BoolVar(&skipChromiumBuild, "skip-chromium", false, "if you want to avoid doing chromium builds (still need the initial build though) for a period of time you can enable this")
	rootCmd.Flags().BoolVar(&remove, "remove", false, "cleanup/destroy all deployed aws resources.")
	rootCmd.Flags().BoolVar(&preventShutdown, "prevent-shutdown", false, "for debugging purposes only - will prevent ec2 instance from shutting down after build.")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(-1)
	}
}

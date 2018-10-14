package main

import (
	"errors"
	"os"

	"github.com/dan-v/rattlesnakeos-stack/stack"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var version string
var name, region, device, sshKey, maxPrice, skipPrice, schedule, instanceType, instanceRegions, repoPatches, repoPrebuilts, hostsFile, chromiumVersion, ami string
var remove, preventShutdown, force, encryptedKeys bool

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
			Name:            name,
			Region:          region,
			Device:          device,
			InstanceType:    instanceType,
			InstanceRegions: instanceRegions,
			SSHKey:          sshKey,
			SkipPrice:       skipPrice,
			MaxPrice:        maxPrice,
			PreventShutdown: preventShutdown,
			Version:         version,
			Schedule:        schedule,
			Force:           force,
			ChromiumVersion: chromiumVersion,
			RepoPatches:     repoPatches,
			RepoPrebuilts:   repoPrebuilts,
			HostsFile:       hostsFile,
			EncryptedKeys:   encryptedKeys,
			AMI:             ami,
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
	rootCmd.Flags().StringVarP(&name, "name", "n", "",
		"name for stack. note: this must be a valid/unique S3 bucket name.")
	rootCmd.MarkFlagRequired("name")
	rootCmd.Flags().StringVarP(&region, "region", "r", "",
		"aws region for stack deployment (e.g. us-west-2)")
	rootCmd.MarkFlagRequired("region")
	rootCmd.Flags().StringVarP(&device, "device", "d", "",
		"device you want to build for: 'marlin' (Pixel XL), 'sailfish' (Pixel), 'taimen' (Pixel 2 XL), 'walleye' (Pixel 2)")
	rootCmd.MarkFlagRequired("device")
	rootCmd.Flags().StringVar(&sshKey, "ssh-key", "",
		"aws ssh key to add to ec2 spot instances. this is optional but is useful for debugging build issues on the instance.")
	rootCmd.Flags().StringVar(&skipPrice, "skip-price", "0.68",
		"skip requesting ec2 spot instance if price is above this value to begin with.")
	rootCmd.Flags().StringVar(&maxPrice, "max-price", "1.00",
		"max ec2 spot instance bid. if this value is too low, you may not obtain an instance or it may terminate during a build.")
	rootCmd.Flags().StringVar(&instanceType, "instance-type", "c5.4xlarge",
		"EC2 instance type (e.g. c4.4xlarge) to use for the build.")
	rootCmd.Flags().StringVar(&instanceRegions, "instance-regions", "us-west-2,us-west-1,us-east-1,us-east-2",
		"possible regions to launch spot instance. the region with cheapest spot instance price will be used.")
	rootCmd.Flags().StringVar(&schedule, "schedule", "rate(14 days)",
		"cron expression that defines when to kick off builds. note: if you give invalid expression it will fail to deploy stack. "+
			"see: https://docs.aws.amazon.com/AmazonCloudWatch/latest/events/ScheduledEvents.html#CronExpressions")
	rootCmd.Flags().StringVar(&repoPatches, "repo-patches", "",
		"an advanced option that allows you to specify a git repo with patches to apply to AOSP build tree. see "+
			"https://github.com/RattlesnakeOS/community_patches for more details.")
	rootCmd.Flags().StringVar(&repoPrebuilts, "repo-prebuilts", "",
		"an advanced option that allows you to specify a git repo with prebuilt APKs. see "+
			"https://github.com/RattlesnakeOS/example_prebuilts for more details.")
	rootCmd.Flags().StringVar(&hostsFile, "hosts-file", "",
		"an advanced option that allows you to specify a replacement /etc/hosts file to enable global dns adblocking "+
			"(e.g. https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts). note: be careful with this, as you "+
			"1) won't get any sort of notification on blocking 2) if you need to unblock something you'll have to rebuild the OS")
	rootCmd.Flags().StringVar(&chromiumVersion, "chromium-version", "",
		"specify the version of Chromium you want (e.g. 69.0.3497.100) to pin to. if not specified, the latest stable "+
			"version of Chromium is used.")
	rootCmd.Flags().StringVar(&ami, "ami-id", "", "override default AMI image for builds")
	rootCmd.Flags().BoolVar(&encryptedKeys, "encrypted-keys", false, "an advanced option that allows signing keys to "+
		"be stored with symmetric gpg encryption and decrypted into memory during the build process. this option requires "+
		"manual intervention during builds where you will be sent a notification and need to provide the key required for "+
		"decryption over SSH to continue the build process. important: if you have an existing stack - please see the FAQ for how to "+
		"migrate your keys")
	rootCmd.Flags().BoolVar(&force, "force", false,
		"build even if there are no changes in available version of AOSP, Chromium, or F-Droid.")
	rootCmd.Flags().BoolVar(&remove, "remove", false,
		"cleanup/destroy all deployed aws resources.")
	rootCmd.Flags().BoolVar(&preventShutdown, "prevent-shutdown", false,
		"for debugging purposes only - will prevent ec2 instance from shutting down after build.")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(-1)
	}
}

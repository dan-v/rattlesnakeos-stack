package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/dan-v/rattlesnakeos-stack/internal/cloudaws"
	"github.com/dan-v/rattlesnakeos-stack/internal/devices"
	"github.com/dan-v/rattlesnakeos-stack/internal/stack"
	"github.com/dan-v/rattlesnakeos-stack/internal/templates"
	"github.com/dan-v/rattlesnakeos-stack/internal/terraform"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/manifoldco/promptui"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
)

const (
	minimumChromiumVersion = 86
)

var (
	name, region, email, device, sshKey, maxPrice, skipPrice, schedule, cloud string
	instanceType, instanceRegions, chromiumVersion, latestURL string
	skipDeploy, chromiumBuildDisabled bool
	supportedDevicesFriendly = devices.GetDeviceFriendlyNames()
	supportedDevicesCodename = devices.GetDeviceCodeNames()
	supportDevicesOutput string
	coreConfigRepo, customConfigRepo string
)

func init() {
	rootCmd.AddCommand(deployCmd)

	for i, d := range supportedDevicesCodename {
		supportDevicesOutput += fmt.Sprintf("%v (%v)", d, supportedDevicesFriendly[i])
		if i < len(supportedDevicesCodename)-1 {
			supportDevicesOutput += ", "
		}
	}

	flags := deployCmd.Flags()

	flags.StringVarP(&name, "name", "n", "",
		"name for stack. note: this must be a valid/unique S3 bucket name.")
	_ = viper.BindPFlag("name", flags.Lookup("name"))

	flags.StringVarP(&region, "region", "r", "",
		"aws region for stack deployment (e.g. us-west-2)")
	_ = viper.BindPFlag("region", flags.Lookup("region"))

	flags.StringVarP(&device, "device", "d", "",
		"device you want to build for (e.g. crosshatch)")
	_ = viper.BindPFlag("device", flags.Lookup("device"))

	flags.StringVarP(&email, "email", "e", "",
		"email address you want to use for build notifications")
	_ = viper.BindPFlag("email", flags.Lookup("email"))

	flags.StringVar(&sshKey, "ssh-key", "",
		"aws ssh key to add to ec2 spot instances. this is optional but is useful for debugging build issues on the instance.")
	_ = viper.BindPFlag("ssh-key", flags.Lookup("ssh-key"))

	flags.StringVar(&skipPrice, "skip-price", "0.68",
		"skip requesting ec2 spot instance if price is above this value to begin with.")
	_ = viper.BindPFlag("skip-price", flags.Lookup("skip-price"))

	flags.StringVar(&maxPrice, "max-price", "1.00",
		"max ec2 spot instance price. if this value is too low, you may not obtain an instance or it may terminate during a build.")
	_ = viper.BindPFlag("max-price", flags.Lookup("max-price"))

	flags.StringVar(&instanceType, "instance-type", "c5.4xlarge", "EC2 instance type (e.g. c5.4xlarge) to use for the build.")
	_ = viper.BindPFlag("instance-type", flags.Lookup("instance-type"))

	flags.StringVar(&instanceRegions, "instance-regions", cloudaws.DefaultInstanceRegions,
		"possible regions to launch spot instance. the region with cheapest spot instance price will be used.")
	_ = viper.BindPFlag("instance-regions", flags.Lookup("instance-regions"))

	flags.StringVar(&schedule, "schedule", "cron(0 0 10 * ? *)",
		"cron expression that defines when to kick off builds. by default this is set to build on the 10th of every month. "+
			"note: if you give an invalid expression it will fail to deploy the stack. "+
			"see this for cron format details: https://docs.aws.amazon.com/AmazonCloudWatch/latest/events/ScheduledEvents.html#CronExpressions")
	_ = viper.BindPFlag("schedule", flags.Lookup("schedule"))

	flags.BoolVar(&chromiumBuildDisabled, "chromium-build-disabled", false, "control whether chromium builds are enabled or disabled.")
	_ = viper.BindPFlag("chromium-build-disabled", flags.Lookup("chromium-build-disabled"))

	flags.StringVar(&chromiumVersion, "chromium-version", "",
		"specify the version of Chromium you want (e.g. 80.0.3971.4) to pin to. if not specified, the latest stable version of Chromium is used.")
	_ = viper.BindPFlag("chromium-version", flags.Lookup("chromium-version"))

	flags.StringVar(&coreConfigRepo, "core-config-repo", templates.DefaultCoreConfigRepo, "a specially formatted repo that contains core customizations on top of AOSP.")
	_ = viper.BindPFlag("core-config-repo", flags.Lookup("core-config-repo"))

	flags.StringVar(&customConfigRepo, "custom-config-repo", "", "a specially formatted repo that contains customizations on top of core.")
	_ = viper.BindPFlag("custom-config-repo", flags.Lookup("custom-config-repo"))

	flags.StringVar(&latestURL, "latest-url", templates.DefaultLatestURL, "url that is used to check versions of aosp/chromium and whether build is required.")
	_ = viper.BindPFlag("latest-url", flags.Lookup("latest-url"))

	flags.StringVar(&cloud, "cloud", "aws", "cloud (aws only right now)")
	_ = viper.BindPFlag("cloud", flags.Lookup("cloud"))

	flags.BoolVar(&skipDeploy, "skip-deploy", false, "only generate the output, but do not deploy with terraform.")
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploy or update the cloud infrastructure used for OS building",
	Args: func(cmd *cobra.Command, args []string) error {
		if viper.GetString("name") == "" {
			return fmt.Errorf("must provide a stack name")
		}
		if viper.GetString("region") == "" {
			return fmt.Errorf("must provide a region")
		}
		if viper.GetString("email") == "" {
			return errors.New("must specify email")
		}
		if viper.GetString("ssh-key") == "" {
			return fmt.Errorf("must provide ssh key name")
		}
		if viper.GetString("device") == "" {
			return errors.New("must specify device type")
		}
		if viper.GetString("chromium-version") != "" {
			chromiumVersionSplit := strings.Split(viper.GetString("chromium-version"), ".")
			if len(chromiumVersionSplit) != 4 {
				return errors.New("invalid chromium-version specified")
			}
			chromiumMajorNumber, err := strconv.Atoi(chromiumVersionSplit[0])
			if err != nil {
				return fmt.Errorf("unable to parse specified chromium-version: %v", err)
			}
			if chromiumMajorNumber < minimumChromiumVersion {
				return fmt.Errorf("pinned chromium-version must have major version of at least %v", minimumChromiumVersion)
			}
		}
		for _, device := range supportedDevicesCodename {
			if device == viper.GetString("device") {
				return nil
			}
		}
		return fmt.Errorf("must specify a supported device: %v", strings.Join(supportedDevicesCodename, ", "))
	},
	Run: func(cmd *cobra.Command, args []string) {
		c := viper.AllSettings()
		bs, err := yaml.Marshal(c)
		if err != nil {
			log.Fatalf("unable to marshal config to YAML: %v", err)
		}
		log.Println("Current settings:")
		fmt.Println(string(bs))

		if !skipDeploy {
			prompt := promptui.Prompt{
				Label:     "Do you want to continue ",
				IsConfirm: true,
			}
			_, err = prompt.Run()
			if err != nil {
				log.Fatalf("exiting: %v", err)
			}
		}

		// TODO: make this configurable
		outputDirFullPath, err := filepath.Abs(fmt.Sprintf("output_%v", viper.GetString("name")))
		if err != nil {
			log.Fatal(err)
		}

		if err := os.MkdirAll(outputDirFullPath, os.ModePerm); err != nil {
			log.Fatal(err)
		}

		templateConfig := &templates.Config{
			Version:               version,
			Name:                  viper.GetString("name"),
			Region:                viper.GetString("region"),
			Device:                viper.GetString("device"),
			DeviceDetails:         devices.GetDeviceDetails(viper.GetString("device")),
			Email:                 viper.GetString("email"),
			InstanceType:          viper.GetString("instance-type"),
			InstanceRegions:       viper.GetString("instance-regions"),
			SkipPrice:             viper.GetString("skip-price"),
			MaxPrice:              viper.GetString("max-price"),
			SSHKey:                viper.GetString("ssh-key"),
			Schedule:              viper.GetString("schedule"),
			ChromiumBuildDisabled: viper.GetBool("chromium-build-disabled"),
			ChromiumVersion:       viper.GetString("chromium-version"),
			CoreConfigRepo:        viper.GetString("core-config-repo"),
			CustomConfigRepo:      viper.GetString("custom-config-repo"),
			LatestURL:             viper.GetString("latest-url"),
			Cloud:                 viper.GetString("cloud"),
		}

		templateRenderer, err := templates.New(templateConfig, templatesFiles, outputDirFullPath)
		if err != nil {
			log.Fatalf("failed to create template client: %v", err)
		}

		awsSetupClient, err := cloudaws.NewSetupClient(
			viper.GetString("name"),
			viper.GetString("region"),
		)
		if err != nil {
			log.Fatalf("failed to create aws setup client: %v", err)
		}

		awsSubscribeClient, err := cloudaws.NewSubscribeClient(
			viper.GetString("name"),
			viper.GetString("region"),
			viper.GetString("email"),
		)
		if err != nil {
			log.Fatalf("failed to create aws subscribe client: %v", err)
		}

		terraformClient, err := terraform.New(outputDirFullPath)
		if err != nil {
			log.Fatalf("failed to create terraform client: %v", err)
		}

		s := stack.New(viper.GetString("name"), templateRenderer, awsSetupClient, awsSubscribeClient, terraformClient)
		if err != nil {
			log.Fatal(err)
		}

		if !skipDeploy {
			ctx, cancel := context.WithTimeout(context.Background(), stack.DefaultDeployTimeout)
			defer cancel()

			if err := s.Deploy(ctx); err != nil {
				log.Fatal(err)
			}
		} else {
			log.Println("skipping deployment as --skip-deploy was specified")
		}
	},
}

package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/dan-v/rattlesnakeos-stack/internal/cloudaws"
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
	instanceType, instanceRegions, chromiumVersion, releasesURL               string
	saveConfig, dryRun, chromiumBuildDisabled                                 bool
	coreConfigRepo, customConfigRepo                                          string
	coreConfigRepoBranch, customConfigRepoBranch                              string
	outputDir                                                                 string
)

func deployInit() {
	rootCmd.AddCommand(deployCmd)

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
		"cron expression that defines when to kick off builds. by default this is set to build on the 10th of every month. you can also set to empty string to disable cron."+
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

	flags.StringVar(&coreConfigRepoBranch, "core-config-repo-branch", aospVersion, "the branch to use for the core config repo.")
	_ = viper.BindPFlag("core-config-repo-branch", flags.Lookup("core-config-repo-branch"))

	flags.StringVar(&customConfigRepo, "custom-config-repo", "", "a specially formatted repo that contains customizations on top of core.")
	_ = viper.BindPFlag("custom-config-repo", flags.Lookup("custom-config-repo"))

	flags.StringVar(&customConfigRepoBranch, "custom-config-repo-branch", "", "the branch to use for the custom config repo. if left blanked the default branch will be checked out.")
	_ = viper.BindPFlag("custom-config-repo-branch", flags.Lookup("custom-config-repo-branch"))

	flags.StringVar(&releasesURL, "releases-url", fmt.Sprintf(templates.DefaultReleasesURLTemplate, aospVersion), "url that is used to check versions of aosp/chromium and whether build is required.")
	_ = viper.BindPFlag("releases-url", flags.Lookup("releases-url"))

	flags.StringVar(&cloud, "cloud", "aws", "cloud (aws only right now)")
	_ = viper.BindPFlag("cloud", flags.Lookup("cloud"))

	flags.StringVar(&outputDir, "output-dir", "", "where to generate all files used for the deployment")
	_ = viper.BindPFlag("output-dir", flags.Lookup("output-dir"))

	flags.BoolVar(&saveConfig, "save-config", false, "allows you to save all passed CLI flags to config file")

	flags.BoolVar(&dryRun, "dry-run", false, "only generate the output files, but do not deploy with terraform.")
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
		if !supportedDevices.IsSupportedDevice(viper.GetString("device")) {
			return fmt.Errorf("must specify a supported device: %v", strings.Join(supportedDevices.GetDeviceCodeNames(), ", "))
		}

		// deprecated checks
		if viper.GetBool("encrypted-keys") {
			return fmt.Errorf("encrypted-keys functionality has been removed (it may return in the future). migration required to use non encrypted keys for now")
		}
		if viper.GetString("core-config-repo-branch") != aospVersion {
			log.Warnf("core-config-repo-branch '%v' does not match aosp version '%v' - if this is not intended, update your config file",
				viper.GetString("core-config-repo-branch"), aospVersion)
		}
		if viper.GetString("hosts-file") != "" {
			log.Warn("hosts-file functionality has been removed - it can be removed from config file")
		}
		if viper.Get("custom-manifest-remotes") != nil {
			return fmt.Errorf("custom-manifest-remotes has been deprecated in favor of custom-config-repo option")
		}
		if viper.Get("custom-manifest-projects") != nil {
			return fmt.Errorf("custom-manifest-projects has been deprecated in favor of custom-config-repo option")
		}
		if viper.Get("custom-patches") != nil {
			return fmt.Errorf("custom-patches has been deprecated in favor of custom-config-repo option")
		}
		if viper.Get("custom-prebuilts") != nil {
			return fmt.Errorf("custom-prebuilts has been deprecated in favor of custom-config-repo option")
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		c := viper.AllSettings()
		bs, err := yaml.Marshal(c)
		if err != nil {
			log.Fatalf("unable to marshal config to YAML: %v", err)
		}
		log.Println("Current settings:")
		fmt.Println(string(bs))

		if !dryRun {
			prompt := promptui.Prompt{
				Label:     "Do you want to continue ",
				IsConfirm: true,
			}
			_, err = prompt.Run()
			if err != nil {
				log.Fatalf("exiting: %v", err)
			}
		}

		configuredOutputDir, err := getOutputDir()
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("all generated files will be placed in %v", configuredOutputDir)

		templateConfig := &templates.Config{
			Version:                stackVersion,
			Name:                   viper.GetString("name"),
			Region:                 viper.GetString("region"),
			Device:                 viper.GetString("device"),
			DeviceDetails:          supportedDevices.GetDeviceDetails(viper.GetString("device")),
			Email:                  viper.GetString("email"),
			InstanceType:           viper.GetString("instance-type"),
			InstanceRegions:        viper.GetString("instance-regions"),
			SkipPrice:              viper.GetString("skip-price"),
			MaxPrice:               viper.GetString("max-price"),
			SSHKey:                 viper.GetString("ssh-key"),
			Schedule:               viper.GetString("schedule"),
			ChromiumBuildDisabled:  viper.GetBool("chromium-build-disabled"),
			ChromiumVersion:        viper.GetString("chromium-version"),
			CoreConfigRepo:         viper.GetString("core-config-repo"),
			CoreConfigRepoBranch:   viper.GetString("core-config-repo-branch"),
			CustomConfigRepo:       viper.GetString("custom-config-repo"),
			CustomConfigRepoBranch: viper.GetString("custom-config-repo-branch"),
			ReleasesURL:            viper.GetString("releases-url"),
			Cloud:                  viper.GetString("cloud"),
		}

		templateRenderer, err := templates.New(templateConfig, templatesFiles, configuredOutputDir)
		if err != nil {
			log.Fatalf("failed to create template client: %v", err)
		}

		if saveConfig {
			log.Printf("Saved settings to config file %v.", configFileFullPath)
			err := viper.WriteConfigAs(configFileFullPath)
			if err != nil {
				log.Fatalf("Failed to write config file %v", configFileFullPath)
			}
		}

		if dryRun {
			log.Infof("rendering all templates to '%v'", configuredOutputDir)
			err = templateRenderer.RenderAll()
			if err != nil {
				log.Fatal(err)
			}
			log.Info("skipping deployment as skip deploy option was specified")
			return
		}
		if viper.GetString("cloud") != "aws" {
			log.Fatal("'aws' is only supported option for cloud at the moment")
		}

		configFileFullPath, err := filepath.Abs(cfgFile)
		if err != nil {
			log.Fatal(err)
		}
		awsSetupClient, err := cloudaws.NewSetupClient(
			viper.GetString("name"),
			viper.GetString("region"),
			configFileFullPath,
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

		terraformClient, err := terraform.New(configuredOutputDir)
		if err != nil {
			log.Fatalf("failed to create terraform client: %v", err)
		}

		s := stack.New(viper.GetString("name"), templateRenderer, awsSetupClient, awsSubscribeClient, terraformClient)
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), stack.DefaultDeployTimeout)
		defer cancel()

		if err := s.Deploy(ctx); err != nil {
			log.Fatal(err)
		}
	},
}

func getOutputDir() (string, error) {
	configuredOutputDir := viper.GetString("output-dir")
	if configuredOutputDir == "" {
		configuredOutputDir = fmt.Sprintf("output_%v", viper.GetString("name"))
	}
	configuredOutputDir, err := filepath.Abs(configuredOutputDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(configuredOutputDir, os.ModePerm); err != nil {
		log.Fatal(err)
	}
	return configuredOutputDir, nil
}
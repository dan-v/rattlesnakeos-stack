package cmd

import (
	"errors"
	"fmt"
	"github.com/dan-v/rattlesnakeos-stack/internal/aws"
	"github.com/dan-v/rattlesnakeos-stack/internal/devices"
	"github.com/dan-v/rattlesnakeos-stack/internal/stack"
	"os"
	"strconv"
	"strings"

	"github.com/manifoldco/promptui"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
)

var name, region, email, device, sshKey, maxPrice, skipPrice, schedule string
var instanceType, instanceRegions, hostsFile, chromiumVersion string
var encryptedKeys, saveConfig bool
var patches = &stack.CustomPatches{}
var scripts = &stack.CustomScripts{}
var prebuilts = &stack.CustomPrebuilts{}
var manifestRemotes = &stack.CustomManifestRemotes{}
var manifestProjects = &stack.CustomManifestProjects{}

var supportedDevicesFriendly = devices.SupportedDevices.GetDeviceFriendlyNames()
var supportedDevicesCodename = devices.SupportedDevices.GetDeviceNames()
var supportDevicesOutput string

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
		"device you want to build for (e.g. crosshatch): to list supported devices use '-d list'")
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

	flags.StringVar(&instanceType, "instance-type", "c5.4xlarge", "EC2 instance type (e.g. c4.4xlarge) to use for the build.")
	_ = viper.BindPFlag("instance-type", flags.Lookup("instance-type"))

	flags.StringVar(&instanceRegions, "instance-regions", aws.DefaultInstanceRegions,
		"possible regions to launch spot instance. the region with cheapest spot instance price will be used.")
	_ = viper.BindPFlag("instance-regions", flags.Lookup("instance-regions"))

	flags.StringVar(&schedule, "schedule", "cron(0 0 10 * ? *)",
		"cron expression that defines when to kick off builds. by default this is set to build on the 10th of every month. "+
			"note: if you give an invalid expression it will fail to deploy the stack. "+
			"see this for cron format details: https://docs.aws.amazon.com/AmazonCloudWatch/latest/events/ScheduledEvents.html#CronExpressions")
	_ = viper.BindPFlag("schedule", flags.Lookup("schedule"))

	flags.StringVar(&hostsFile, "hosts-file", "",
		"an advanced option that allows you to specify a replacement /etc/hosts file to enable global dns adblocking "+
			"(e.g. https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts). note: be careful with this, as you "+
			"1) won't get any sort of notification on blocking 2) if you need to unblock something you'll have to rebuild the OS")
	_ = viper.BindPFlag("hosts-file", flags.Lookup("hosts-file"))

	flags.StringVar(&chromiumVersion, "chromium-version", "",
		"specify the version of Chromium you want (e.g. 80.0.3971.4) to pin to. if not specified, the latest stable "+
			"version of Chromium is used.")
	_ = viper.BindPFlag("chromium-version", flags.Lookup("chromium-version"))

	flags.BoolVar(&encryptedKeys, "encrypted-keys", false, "an advanced option that allows signing keys to "+
		"be stored with symmetric gpg encryption and decrypted into memory during the build process. this option requires "+
		"manual intervention during builds where you will be sent a notification and need to provide the key required for "+
		"decryption over SSH to continue the build process. important: if you have an existing stack - please see the FAQ for how to "+
		"migrate your keys")
	_ = viper.BindPFlag("encrypted-keys", flags.Lookup("encrypted-keys"))
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy or update the AWS infrastructure used for building RattlesnakeOS",
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
			if chromiumMajorNumber < stack.MinimumChromiumVersion {
				return fmt.Errorf("pinned chromium-version must have major version of at least %v", stack.MinimumChromiumVersion)
			}
		}
		if device == "list" {
			fmt.Printf("Valid devices are: %v\n", supportDevicesOutput)
			os.Exit(0)
		}
		for _, device := range supportedDevicesCodename {
			if device == viper.GetString("device") {
				return nil
			}
		}
		return fmt.Errorf("must specify a supported device: %v", strings.Join(supportedDevicesCodename, ", "))
	},
	Run: func(cmd *cobra.Command, args []string) {
		viper.UnmarshalKey("custom-patches", patches)
		viper.UnmarshalKey("custom-scripts", scripts)
		viper.UnmarshalKey("custom-prebuilts", prebuilts)
		viper.UnmarshalKey("custom-manifest-remotes", manifestRemotes)
		viper.UnmarshalKey("custom-manifest-projects", manifestProjects)

		c := viper.AllSettings()
		bs, err := yaml.Marshal(c)
		if err != nil {
			log.Fatalf("unable to marshal config to YAML: %v", err)
		}
		log.Println("Current settings:")
		fmt.Println(string(bs))

		for _, r := range *patches {
			if !strings.Contains(strings.ToLower(r.Repo), stack.DefaultTrustedRepoBase) {
				log.Warnf("You are using an untrusted repository (%v) for patches - this is risky unless you own the repository", r.Repo)
			}
		}

		for _, r := range *scripts {
			if !strings.Contains(strings.ToLower(r.Repo), stack.DefaultTrustedRepoBase) {
				log.Warnf("You are using an untrusted repository (%v) for scripts - this is risky unless you own the repository", r.Repo)
			}
		}

		for _, r := range *prebuilts {
			if !strings.Contains(strings.ToLower(r.Repo), stack.DefaultTrustedRepoBase) {
				log.Warnf("You are using an untrusted repository (%v) for prebuilts - this is risky unless you own the repository", r.Repo)
			}
		}

		prompt := promptui.Prompt{
			Label:     "Do you want to continue ",
			IsConfirm: true,
		}
		_, err = prompt.Run()
		if err != nil {
			log.Fatalf("Exiting %v", err)
		}

		s, err := stack.New(&stack.Config{
			Name:                   viper.GetString("name"),
			Region:                 viper.GetString("region"),
			Device:                 viper.GetString("device"),
			DeviceDetails:          devices.GetDeviceDetails(viper.GetString("device")),
			Email:                  viper.GetString("email"),
			InstanceType:           viper.GetString("instance-type"),
			InstanceRegions:        viper.GetString("instance-regions"),
			SSHKey:                 viper.GetString("ssh-key"),
			SkipPrice:              viper.GetString("skip-price"),
			MaxPrice:               viper.GetString("max-price"),
			Schedule:               viper.GetString("schedule"),
			ChromiumVersion:        viper.GetString("chromium-version"),
			HostsFile:              viper.GetString("hosts-file"),
			EncryptedKeys:          viper.GetBool("encrypted-keys"),
			CustomPatches:          patches,
			CustomScripts:          scripts,
			CustomPrebuilts:        prebuilts,
			CustomManifestRemotes:  manifestRemotes,
			CustomManifestProjects: manifestProjects,
			Version:                version,
		}, buildScript, buildTemplate, lambdaTemplate, terraformTemplate)
		if err != nil {
			log.Fatal(err)
		}
		if err := s.Apply(); err != nil {
			log.Fatal(err)
		}
	},
}

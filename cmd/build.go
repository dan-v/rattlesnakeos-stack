package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dan-v/rattlesnakeos-stack/internal/cloudaws"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"time"
)

var (
	terminateInstanceID, terminateRegion, listRegions string
	aospBuildID, aospTag string
	forceBuild bool
	defaultExecuteLambdaTimeout = time.Second * 200
	defaultTerminateInstanceTimeout = time.Second * 10
	defaultListInstancesTimeout = time.Second * 10
)

func init() {
	rootCmd.AddCommand(buildCmd)

	buildCmd.AddCommand(buildListCmd)
	buildListCmd.Flags().StringVar(&name, "name", "", "name for stack")
	buildListCmd.Flags().StringVar(&listRegions, "instance-regions", "", "regions to look for running builds")

	buildCmd.AddCommand(buildStartCmd)
	buildStartCmd.Flags().StringVar(&name, "name", "", "name of stack")
	buildStartCmd.Flags().BoolVar(&forceBuild, "force-build", false, "force build even if there are no changes in component versions")
	buildStartCmd.Flags().StringVar(&aospBuildID, "aosp-build-id", "", "advanced option - specify the specific the AOSP build id (e.g. RQ1A.210205.004)")
	buildStartCmd.Flags().StringVar(&aospTag, "aosp-tag", "", "advanced option - specify the corresponding AOSP tag to use for build (e.g. android-11.0.0_r29)")

	buildCmd.AddCommand(buildTerminateCmd)
	buildTerminateCmd.Flags().StringVarP(&terminateInstanceID, "instance-id", "i", "", "EC2 instance id "+
		"you want to terminate (e.g. i-07ff0f2ed84ff2e8d)")
	buildTerminateCmd.Flags().StringVarP(&terminateRegion, "region", "r", "", "Region of instance you "+
		"want to terminate")
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "commands to list, start, and terminate builds.",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("Need to specify a subcommand")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {},
}

var buildStartCmd = &cobra.Command{
	Use:   "start",
	Short: "manually start a build",
	Args: func(cmd *cobra.Command, args []string) error {
		if viper.GetString("name") == "" && name == "" {
			return fmt.Errorf("must provide a stack name")
		}
		if viper.GetString("region") == "" && region == "" {
			return fmt.Errorf("must provide stack region")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if name == "" {
			name = viper.GetString("name")
		}
		if region == "" {
			region = viper.GetString("region")
		}

		payload, err := json.Marshal(struct {
			ForceBuild  bool   `json:"force-build"`
			AOSPBuildID string `json:"aosp-build-id"`
			AOSPTag     string `json:"aosp-tag"`
		}{
			ForceBuild:  forceBuild,
			AOSPBuildID: aospBuildID,
			AOSPTag:     aospTag,
		})
		if err != nil {
			log.Fatalf("failed to create payload for lambda function: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), defaultExecuteLambdaTimeout)
		defer cancel()

		log.Infof("calling lambda function to start manual build for stack %v. waiting for spot instance launch...", name)
		output, err := cloudaws.ExecuteLambdaFunction(ctx, name, region, payload)
		if err != nil {
			log.Fatalf("failed to start manual build for stack %v: err=%v", name, err)
		}
		if output != nil && (output.FunctionError != nil || output.StatusCode != 200)  {
			log.Fatalf("failed to start manual build for stack %v: statuscode=%v payload:%v",
				name, output.StatusCode, output.Payload)
		}

		log.Infof("successfully started manual build for stack %v", name)
	},
}

var buildTerminateCmd = &cobra.Command{
	Use:   "terminate",
	Short: "terminate a running a build",
	Args: func(cmd *cobra.Command, args []string) error {
		if terminateInstanceID == "" {
			return fmt.Errorf("must provide an instance id to terminate")
		}
		if terminateRegion == "" {
			return fmt.Errorf("must provide region for instance to terminate")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTerminateInstanceTimeout)
		defer cancel()

		output, err := cloudaws.TerminateEC2Instance(ctx, terminateInstanceID, terminateRegion)
		if err != nil {
			log.Fatal(err)
		}

		log.Infof("terminated instance %v in region %v: %v", terminateInstanceID, terminateRegion, output.TerminatingInstances)
	},
}

var buildListCmd = &cobra.Command{
	Use:   "list",
	Short: "list in progress builds",
	Args: func(cmd *cobra.Command, args []string) error {
		if viper.GetString("name") == "" && name == "" {
			return fmt.Errorf("must provide a stack name")
		}
		if viper.GetString("instance-regions") == "" && listRegions == "" {
			return fmt.Errorf("must provide instance regions")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if name == "" {
			name = viper.GetString("name")
		}
		if listRegions == "" {
			listRegions = viper.GetString("instance-regions")
		}

		ctx, cancel := context.WithTimeout(context.Background(), defaultListInstancesTimeout)
		defer cancel()

		instances, err := cloudaws.GetRunningEC2InstancesWithProfileName(ctx, fmt.Sprintf("%v-ec2", name), listRegions)
		if err != nil {
			log.Fatal(err)
		}

		if len(instances) == 0 {
			log.Info("no active builds found")
			return
		}

		for _, instance := range instances {
			fmt.Println(instance)
		}
	},
}

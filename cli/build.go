package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/service/lambda"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// TODO: this command is very happy path at the moment

var listBuilds, startBuild, forceBuild bool
var terminateInstanceID, terminateRegion, listRegions, listName, buildName string
var aospBuild, aospBranch string

func init() {
	rootCmd.AddCommand(buildCmd)

	buildCmd.AddCommand(buildListCmd)
	buildListCmd.Flags().StringVar(&name, "name", "", "name for stack")
	buildListCmd.Flags().StringVar(&listRegions, "instance-regions", "", "regions to look for running builds")

	buildCmd.AddCommand(buildStartCmd)
	buildStartCmd.Flags().StringVar(&name, "name", "", "name for stack")
	buildStartCmd.Flags().BoolVar(&forceBuild, "force-build", false, "force build even if there are no changes in "+
		"available version of AOSP, Chromium, or F-Droid. this will override stack setting ignore-version-checks.")
	buildStartCmd.Flags().StringVar(&aospBuild, "aosp-build", "", "advanced option - specify the specific factory image build number (e.g. PQ3A.190505.002)")
	buildStartCmd.Flags().StringVar(&aospBranch, "aosp-branch", "", "advanced option - specify the corresponding AOSP branch to use for build (e.g. android-9.0.0_r37)")

	buildCmd.AddCommand(buildTerminateCmd)
	buildTerminateCmd.Flags().StringVarP(&terminateInstanceID, "instance-id", "i", "", "EC2 instance id "+
		"you want to terminate (e.g. i-07ff0f2ed84ff2e8d)")
	buildTerminateCmd.Flags().StringVarP(&terminateRegion, "region", "r", "", "Region of instance you "+
		"want to terminate")
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Commands to list, start, and terminate builds.",
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
	Short: "Manually start a build",
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

		sess, err := session.NewSession(aws.NewConfig().WithCredentialsChainVerboseErrors(true))
		if err != nil {
			log.Fatalf("Failed to setup AWS session: %v", err)
		}

		lambdaPayload := struct {
			ForceBuild bool
			AOSPBuild  string
			AOSPBranch string
		}{
			ForceBuild: forceBuild,
			AOSPBuild:  aospBuild,
			AOSPBranch: aospBranch,
		}
		payload, err := json.Marshal(lambdaPayload)
		if err != nil {
			log.Fatalf("Failed to create payload for Lambda function: %v", err)
		}

		lambdaClient := lambda.New(sess, &aws.Config{Region: &region})
		out, err := lambdaClient.Invoke(&lambda.InvokeInput{
			FunctionName:   aws.String(name + "-build"),
			InvocationType: aws.String("RequestResponse"),
			Payload:        payload,
		})
		if err != nil {
			log.Fatalf("Failed to start manual build: %v", err)
		}
		if out.FunctionError != nil {
			log.Fatalf("Failed to start manual build. Function error: %v. Output: %v", *out.FunctionError, string(out.Payload))
		}
		if *out.StatusCode != 200 {
			log.Fatalf("Failed to start manual build. Status code calling Lambda function %v != 200", *out.StatusCode)
		}
		log.Infof("Successfully started manual build for stack %v", name)
	},
}

var buildTerminateCmd = &cobra.Command{
	Use:   "terminate",
	Short: "Terminate a running a build",
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
		sess, err := session.NewSession(aws.NewConfig().WithCredentialsChainVerboseErrors(true))
		if err != nil {
			log.Fatalf("Failed to setup AWS session: %v", err)
		}
		ec2Client := ec2.New(sess, &aws.Config{Region: &terminateRegion})
		_, err = ec2Client.TerminateInstances(&ec2.TerminateInstancesInput{
			InstanceIds: aws.StringSlice([]string{terminateInstanceID}),
		})
		if err != nil {
			log.Fatalf("Failed to terminate EC2 instance %v in region %v: %v", terminateInstanceID, terminateRegion, err)
		}
		log.Infof("Terminated instance %v in region %v", terminateInstanceID, terminateRegion)
	},
}

var buildListCmd = &cobra.Command{
	Use:   "list",
	Short: "List in progress RattlesnakeOS builds",
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

		sess, err := session.NewSession(aws.NewConfig().WithCredentialsChainVerboseErrors(true))
		if err != nil {
			log.Fatalf("Failed to setup AWS session: %v", err)
		}

		log.Infof("Looking for builds for stack %v in the following regions: %v", name, instanceRegions)
		runningInstances := 0
		for _, region := range strings.Split(listRegions, ",") {
			ec2Client := ec2.New(sess, &aws.Config{Region: &region})
			resp, err := ec2Client.DescribeInstances(&ec2.DescribeInstancesInput{
				Filters: []*ec2.Filter{
					&ec2.Filter{
						Name:   aws.String("instance-state-name"),
						Values: []*string{aws.String("running")}},
				}})
			if err != nil {
				log.Fatalf("Failed to describe EC2 instances in region %v", region)
			}
			if len(resp.Reservations) == 0 || len(resp.Reservations[0].Instances) == 0 {
				continue
			}

			for _, reservation := range resp.Reservations {
				for _, instance := range reservation.Instances {
					if instance.IamInstanceProfile == nil || instance.IamInstanceProfile.Arn == nil {
						continue
					}

					instanceIamProfileName := strings.Split(*instance.IamInstanceProfile.Arn, "/")[1]
					if instanceIamProfileName == name+"-ec2" {
						log.Printf("Instance '%v': ip='%v' region='%v' launched='%v'", *instance.InstanceId, *instance.PublicIpAddress, region, *instance.LaunchTime)
						runningInstances++
					}
				}
			}
		}
		if runningInstances == 0 {
			log.Info("No active builds found")
		}
	},
}

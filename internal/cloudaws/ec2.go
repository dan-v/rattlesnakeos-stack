package cloudaws

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"strings"
)

// TerminateEC2Instance terminates the specified ec2 instance
func TerminateEC2Instance(ctx context.Context, instanceID, region string) (*ec2.TerminateInstancesOutput, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	ec2Client := ec2.NewFromConfig(cfg)
	output, err := ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to terminate ec2 instance '%v' in region '%v': output:%v error:%v", instanceID, region, output, err)
	}

	return output, nil
}

// GetRunningEC2InstancesWithProfileName returns a list of instances running with a profile name
func GetRunningEC2InstancesWithProfileName(ctx context.Context, profileName, listRegions string) ([]string, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	var instances []string
	for _, region := range strings.Split(listRegions, ",") {
		ec2Client := ec2.NewFromConfig(cfg, func(o *ec2.Options) {
			o.Region = region
		})
		resp, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			Filters: []ec2types.Filter{
				{
					Name:   aws.String("instance-state-name"),
					Values: []string{"running"}},
			},
		},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to describe ec2 instances in region %v", region)
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
				if instanceIamProfileName == profileName {
					instances = append(instances, fmt.Sprintf("instance='%v' ip='%v' region='%v' launched='%v",
						*instance.InstanceId, *instance.PublicIpAddress, region, *instance.LaunchTime))
				}
			}
		}
	}
	return instances, nil
}

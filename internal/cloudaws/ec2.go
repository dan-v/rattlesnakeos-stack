package cloudaws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"strings"
)

func TerminateEC2Instance(instanceID, region string) (*ec2.TerminateInstancesOutput, error) {
	sess, err := getSession()
	if err != nil {
		return nil, fmt.Errorf("failed to get session for terminating ec2 instance: %w", err)
	}

	ec2Client := ec2.New(sess, &aws.Config{Region: &region})
	output, err := ec2Client.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: aws.StringSlice([]string{instanceID}),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to terminate ec2 instance '%v' in region '%v': output:%v error:%v", instanceID, region, output, err)
	}

	return output, nil
}

func GetRunningEC2InstancesWithProfileName(profileName, listRegions string) ([]string, error) {
	sess, err := getSession()
	if err != nil {
		return nil, fmt.Errorf("failed to get session for describing ec2 instance: %w", err)
	}

	instances := []string{}
	for _, region := range strings.Split(listRegions, ",") {
		ec2Client := ec2.New(sess, &aws.Config{Region: &region})
		resp, err := ec2Client.DescribeInstances(&ec2.DescribeInstancesInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("instance-state-name"),
					Values: []*string{aws.String("running")}},
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

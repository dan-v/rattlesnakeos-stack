package stack

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	log "github.com/sirupsen/logrus"
)

const (
	awsErrCodeNoSuchBucket = "NoSuchBucket"
	awsErrCodeNotFound     = "NotFound"
)

// ubuntu 16.04 AMI
var amiMap = map[string]string{
	"ap-northeast-1": "ami-bec974d8",
	"ap-northeast-2": "ami-3066c15e",
	"ap-south-1":     "ami-f3e5aa9c",
	"ap-southeast-1": "ami-10acfb73",
	"ap-southeast-2": "ami-cab258a8",
	"ca-central-1":   "ami-018b3065",
	"eu-central-1":   "ami-df8406b0",
	"eu-west-1":      "ami-8fd760f6",
	"eu-west-2":      "ami-fcc4db98",
	"eu-west-3":      "ami-4262d53f",
	"sa-east-1":      "ami-bf8ecbd3",
	"us-east-1":      "ami-aa2ea6d0",
	"us-east-2":      "ami-82f4dae7",
	"us-west-1":      "ami-45ead225",
	"us-west-2":      "ami-0def3275",
}

type StackConfig struct {
	Name            string
	Region          string
	Device          string
	AMI             string
	SpotPrice       string
	SSHKey          string
	PreventShutdown bool
}

func AWSApply(config StackConfig) error {
	err := checkAWSCreds(config.Region)
	if err != nil {
		return err
	}

	if config.AMI == "" {
		ami, err := getAMI(config.Region)
		if err != nil {
			return err
		}
		config.AMI = ami
	}

	err = s3BucketSetup(config)
	if err != nil {
		return err
	}

	terraformClient, err := generateConfigAndGetClient(config)
	defer terraformClient.Cleanup()

	log.Info("Creating AWS resources")
	err = terraformClient.Apply()
	if err != nil {
		log.Fatalln("Failed to create AWS resources:", err)
	}
	log.Info("Successfully deployed AWS resources")
	return nil
}

func AWSDestroy(config StackConfig) error {
	err := checkAWSCreds(config.Region)
	if err != nil {
		return err
	}

	terraformClient, err := generateConfigAndGetClient(config)
	defer terraformClient.Cleanup()

	log.Info("Destroying AWS resources")
	err = terraformClient.Destroy()
	if err != nil {
		log.Fatalln("Failed to destroy AWS resources:", err)
	}
	log.Info("Successfully removed AWS resources")
	return nil
}

func checkAWSCreds(region string) error {
	log.Info("Checking AWS credentials")
	sess, err := session.NewSession(aws.NewConfig().WithCredentialsChainVerboseErrors(true))
	if err != nil {
		return fmt.Errorf("Failed to create new AWS session: %v", err)
	}
	s3Client := s3.New(sess, &aws.Config{Region: &region})
	_, err = s3Client.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		return fmt.Errorf("Unable to list S3 buckets - make sure you have valid admin AWS credentials")
	}
	return nil
}

func s3BucketSetup(config StackConfig) error {
	sess, err := session.NewSession(aws.NewConfig().WithCredentialsChainVerboseErrors(true))
	if err != nil {
		return fmt.Errorf("Failed to create new AWS session: %v", err)
	}
	s3Client := s3.New(sess, &aws.Config{Region: &config.Region})

	log.Infof("Creating S3 bucket %s", config.Name)
	_, err = s3Client.HeadBucket(&s3.HeadBucketInput{Bucket: &config.Name})
	if err != nil {
		awsErrCode := err.(awserr.Error).Code()
		if awsErrCode != awsErrCodeNotFound && awsErrCode != awsErrCodeNoSuchBucket {
			return fmt.Errorf("Unknown S3 error code: %v", err)
		}

		bucketInput := &s3.CreateBucketInput{
			Bucket: &config.Name,
		}
		// NOTE the location constraint should only be set if using a bucket OTHER than us-east-1
		// http://docs.aws.amazon.com/AmazonS3/latest/API/RESTBucketPUT.html
		if config.Region != "us-east-1" {
			bucketInput.CreateBucketConfiguration = &s3.CreateBucketConfiguration{
				LocationConstraint: &config.Region,
			}
		}

		_, err = s3Client.CreateBucket(bucketInput)
		if err != nil {
			return fmt.Errorf("Failed to create bucket %s - note that this bucket name must be globally unique. %v", config.Name, err)
		}
	}
	return nil
}

func getAMI(region string) (string, error) {
	if _, ok := amiMap[region]; !ok {
		return "", fmt.Errorf("Unknown region %s. Need to manually specify AMI.", region)
	}
	return amiMap[region], nil
}

func generateConfigAndGetClient(config StackConfig) (*TerraformClient, error) {
	terraformConf, err := generateTerraformConfig(config)
	if err != nil {
		return nil, fmt.Errorf("Failed to generate config: %v", err)
	}

	terraformClient, err := NewTerraformClient(terraformConf, os.Stdout, os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("Failed to create client: %v", err)
	}
	return terraformClient, nil
}

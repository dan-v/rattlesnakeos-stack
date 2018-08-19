package stack

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/dan-v/rattlesnakeos-stack/templates"
	log "github.com/sirupsen/logrus"
)

const (
	awsErrCodeNoSuchBucket = "NoSuchBucket"
	awsErrCodeNotFound     = "NotFound"
)

type AWSStackConfig struct {
	Name            string
	Region          string
	Device          string
	AMI             string
	SpotPrice       string
	SSHKey          string
	PreventShutdown bool
	Version         string
	Schedule        string
	Force           bool
}

type AWSStack struct {
	Config                  *AWSStackConfig
	terraformClient         *terraformClient
	renderedBuildScript     []byte
	renderedLambdaFunction  []byte
	LambdaZipFileLocation   string
	BuildScriptFileLocation string
}

func NewAWSStack(config *AWSStackConfig) (*AWSStack, error) {
	err := checkAWSCreds(config.Region)
	if err != nil {
		return nil, err
	}

	err = s3BucketSetup(config.Name, config.Region)
	if err != nil {
		return nil, err
	}

	if config.AMI == "" {
		ami, err := getAMI(config.Region)
		if err != nil {
			return nil, err
		}
		config.AMI = ami
	}

	renderedLambdaFunction, err := renderTemplate(templates.LambdaTemplate, config)
	if err != nil {
		return nil, fmt.Errorf("Failed to render Lambda function: %v", err)
	}

	renderedBuildScript, err := renderTemplate(templates.BuildTemplate, config)
	if err != nil {
		return nil, fmt.Errorf("Failed to render build script: %v", err)
	}

	stack := &AWSStack{
		Config:                 config,
		renderedBuildScript:    renderedBuildScript,
		renderedLambdaFunction: renderedLambdaFunction,
	}

	terraformClient, err := newTerraformClient(stack, os.Stdout, os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("Failed to create terraform client: %v", err)
	}
	stack.terraformClient = terraformClient

	return stack, nil
}

func (s *AWSStack) Apply() error {
	defer s.terraformClient.Cleanup()

	log.Info("Creating AWS resources")
	err := s.terraformClient.Apply()
	if err != nil {
		return err
	}
	log.Info("Successfully deployed AWS resources")
	return nil
}

func (s *AWSStack) Destroy() error {
	defer s.terraformClient.Cleanup()

	log.Info("Destroying AWS resources")
	err := s.terraformClient.Destroy()
	if err != nil {
		return err
	}
	log.Info("Successfully removed AWS resources")
	return nil
}

func s3BucketSetup(name, region string) error {
	sess, err := session.NewSession(aws.NewConfig().WithCredentialsChainVerboseErrors(true))
	if err != nil {
		return fmt.Errorf("Failed to create new AWS session: %v", err)
	}
	s3Client := s3.New(sess, &aws.Config{Region: &region})

	log.Infof("Creating S3 bucket %s", name)
	_, err = s3Client.HeadBucket(&s3.HeadBucketInput{Bucket: &name})
	if err != nil {
		awsErrCode := err.(awserr.Error).Code()
		if awsErrCode != awsErrCodeNotFound && awsErrCode != awsErrCodeNoSuchBucket {
			return fmt.Errorf("Unknown S3 error code: %v", err)
		}

		bucketInput := &s3.CreateBucketInput{
			Bucket: &name,
		}
		// NOTE the location constraint should only be set if using a bucket OTHER than us-east-1
		// http://docs.aws.amazon.com/AmazonS3/latest/API/RESTBucketPUT.html
		if region != "us-east-1" {
			bucketInput.CreateBucketConfiguration = &s3.CreateBucketConfiguration{
				LocationConstraint: &region,
			}
		}

		_, err = s3Client.CreateBucket(bucketInput)
		if err != nil {
			return fmt.Errorf("Failed to create bucket %s - note that this bucket name must be globally unique. %v", name, err)
		}
	}
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
		return fmt.Errorf("Unable to list S3 buckets - make sure you have valid admin AWS credentials: %v", err)
	}
	return nil
}

func getAMI(region string) (string, error) {
	if _, ok := amiMap[region]; !ok {
		return "", fmt.Errorf("Unknown region %s. Need to manually specify AMI.", region)
	}
	return amiMap[region], nil
}

// ubuntu 16.04 AMI hvm:ebs-ssd
// https://cloud-images.ubuntu.com/locator/ec2/
var amiMap = map[string]string{
	"ap-northeast-1": "ami-940cdceb",
	"ap-northeast-2": "ami-467acf28",
	"ap-northeast-3": "ami-85b3bdf8",
	"ap-south-1":     "ami-188fba77",
	"ap-southeast-1": "ami-51a7aa2d",
	"ap-southeast-2": "ami-47c21a25",
	"ca-central-1":   "ami-db9e1cbf",
	"cn-north-1":     "ami-b117c9dc",
	"cn-northwest-1": "ami-39b8ac5b",
	"eu-central-1":   "ami-de8fb135",
	"eu-west-1":      "ami-2a7d75c0",
	"eu-west-2":      "ami-6b3fd60c",
	"eu-west-3":      "ami-20ee5e5d",
	"sa-east-1":      "ami-8eecc9e2",
	"us-east-1":      "ami-759bc50a",
	"us-east-2":      "ami-5e8bb23b",
	"us-gov-west-1":  "ami-0661f767",
	"us-west-1":      "ami-4aa04129",
	"us-west-2":      "ami-ba602bc2",
}

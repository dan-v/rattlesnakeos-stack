package stack

import (
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/dan-v/rattlesnakeos-stack/templates"
	log "github.com/sirupsen/logrus"
)

const (
	awsErrCodeNoSuchBucket = "NoSuchBucket"
	awsErrCodeNotFound     = "NotFound"
)

type CustomPatches []struct {
	Repo    string
	Patches []string
}

type CustomScripts []struct {
	Repo    string
	Scripts []string
}

type CustomPrebuilts []struct {
	Repo    string
	Modules []string
}

type CustomManifestRemotes []struct {
	Name     string
	Fetch    string
	Revision string
}

type CustomManifestProjects []struct {
	Path    string
	Name    string
	Remote  string
	Modules []string
}

type AWSStackConfig struct {
	Name                    string
	Region                  string
	Device                  string
	Email                   string
	InstanceType            string
	InstanceRegions         string
	SkipPrice               string
	MaxPrice                string
	SSHKey                  string
	PreventShutdown         bool
	Version                 string
	Schedule                string
	IgnoreVersionChecks     bool
	ChromiumVersion         string
	CustomPatches           *CustomPatches
	CustomScripts           *CustomScripts
	CustomPrebuilts         *CustomPrebuilts
	CustomManifestRemotes   *CustomManifestRemotes
	CustomManifestProjects  *CustomManifestProjects
	HostsFile               string
	EncryptedKeys           bool
	AMI                     string
	EnableAttestation       bool
	AttestationMaxSpotPrice string
	AttestationInstanceType string
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
	log.Infof("Successfully deployed/updated AWS resources for stack %v", s.Config.Name)

	sess, err := session.NewSession(aws.NewConfig().WithCredentialsChainVerboseErrors(true))
	snsClient := sns.New(sess, &aws.Config{Region: &s.Config.Region})
	resp, err := snsClient.ListTopics(&sns.ListTopicsInput{NextToken: aws.String("")})
	for _, topic := range resp.Topics {
		topicName := strings.Split(*topic.TopicArn, ":")[5]
		if topicName == s.Config.Name {
			// check if subscription exists
			resp, err := snsClient.ListSubscriptionsByTopic(&sns.ListSubscriptionsByTopicInput{
				NextToken: aws.String(""),
				TopicArn:  aws.String(*topic.TopicArn),
			})
			if err != nil {
				return fmt.Errorf("Failed to list SNS subscriptions for topic %v: %v", *topic.TopicArn, err)
			}
			for _, subscription := range resp.Subscriptions {
				if *subscription.Endpoint == s.Config.Email {
					return nil
				}
			}

			// subscribe if not setup
			_, err = snsClient.Subscribe(&sns.SubscribeInput{
				Protocol: aws.String("email"),
				TopicArn: aws.String(*topic.TopicArn),
				Endpoint: aws.String(s.Config.Email),
			})
			if err != nil {
				return fmt.Errorf("Failed to setup email notifications: %v", err)
			}
			log.Infof("Successfully setup email notifications for %v - you'll "+
				"need to click link in confirmation email to get notifications.", s.Config.Email)
			break
		}
	}

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

		log.Infof("Creating S3 bucket %s", name)
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

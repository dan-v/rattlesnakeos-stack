package cloudaws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sns"
	"strings"
)

const (
	// DefaultInstanceRegions is the default regions to look for spot instances
	DefaultInstanceRegions = "us-west-2,us-west-1,us-east-2"
	awsErrCodeNoSuchBucket = "NoSuchBucket"
	awsErrCodeNotFound     = "NotFound"
)

type AWS struct {
	sess         *session.Session
	name         string
	region       string
	topicName    string
	emailAddress string
}

func getSession() (*session.Session, error) {
	return session.NewSession(aws.NewConfig().WithCredentialsChainVerboseErrors(true))
}

func New(name, region, emailAddress string) (*AWS, error) {
	sess, err := getSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create new AWS session: %v", err)
	}

	if err := checkS3Access(sess, region); err != nil {
		return nil, err
	}

	return &AWS{
		sess:         sess,
		name:         name,
		region:       region,
		emailAddress: emailAddress,
	}, nil
}

func (c *AWS) Setup() error {
	if err := c.s3Setup(); err != nil {
		return err
	}
	if err := c.rolesSetup(); err != nil {
		return err
	}
	return nil
}

func (c *AWS) Subscribe() error {
	return c.subscribe()
}

func (c *AWS) subscribe() error {
	snsClient := sns.New(c.sess, &aws.Config{Region: &c.region})
	resp, err := snsClient.ListTopics(&sns.ListTopicsInput{NextToken: aws.String("")})
	if err != nil {
		return fmt.Errorf("failed to list sns topics: %w", err)
	}

	for _, topic := range resp.Topics {
		if c.name == strings.Split(*topic.TopicArn, ":")[5] {
			resp, err := snsClient.ListSubscriptionsByTopic(&sns.ListSubscriptionsByTopicInput{
				NextToken: aws.String(""),
				TopicArn:  aws.String(*topic.TopicArn),
			})
			if err != nil {
				return fmt.Errorf("failed to list SNS subscriptions for topic %v: %w", *topic.TopicArn, err)
			}

			// if subscription already exists return
			for _, subscription := range resp.Subscriptions {
				if *subscription.Endpoint == c.emailAddress {
					return nil
				}
			}

			// subscribe if not setup
			_, err = snsClient.Subscribe(&sns.SubscribeInput{
				Protocol: aws.String("email"),
				TopicArn: aws.String(*topic.TopicArn),
				Endpoint: aws.String(c.emailAddress),
			})
			if err != nil {
				return fmt.Errorf("failed to setup email notifications: %w", err)
			}
			break
		}
	}
	return nil
}

func (c *AWS) rolesSetup() error {
	_, err := iam.New(c.sess).CreateServiceLinkedRole(&iam.CreateServiceLinkedRoleInput{
		AWSServiceName: aws.String("spot.amazonaws.com"),
	})
	if errWithCode, ok := err.(awserr.Error); ok && iam.ErrCodeInvalidInputException != errWithCode.Code() {
		return fmt.Errorf("failed to create spot.amazonaws.com service linked role: %w", errWithCode)
	}

	_, err = iam.New(c.sess).CreateServiceLinkedRole(&iam.CreateServiceLinkedRoleInput{
		AWSServiceName: aws.String("spotfleet.amazonaws.com"),
	})
	if errWithCode, ok := err.(awserr.Error); ok && iam.ErrCodeInvalidInputException != errWithCode.Code() {
		return fmt.Errorf("failed to create spotfleet.amazonaws.com service linked role: %w", errWithCode)
	}

	return nil
}

func (c *AWS) s3Setup() error {
	s3Client := s3.New(c.sess, &aws.Config{Region: &c.region})
	_, err := s3Client.HeadBucket(&s3.HeadBucketInput{Bucket: &c.name})
	if err != nil {
		awsErrCode := err.(awserr.Error).Code()
		if awsErrCode != awsErrCodeNotFound && awsErrCode != awsErrCodeNoSuchBucket {
			return fmt.Errorf("unknown S3 error code: %w", err)
		}

		bucketInput := &s3.CreateBucketInput{
			Bucket: &c.name,
		}
		// NOTE the location constraint should only be set if using a bucket OTHER than us-east-1
		// http://docs.aws.amazon.com/AmazonS3/latest/API/RESTBucketPUT.html
		if c.region != "us-east-1" {
			bucketInput.CreateBucketConfiguration = &s3.CreateBucketConfiguration{
				LocationConstraint: &c.region,
			}
		}

		_, err = s3Client.CreateBucket(bucketInput)
		if err != nil {
			return fmt.Errorf("failed to create bucket %s - note that this bucket name must be globally unique. %w", c.name, err)
		}
	}
	return nil
}

func checkS3Access(sess *session.Session, region string) error {
	s3Client := s3.New(sess, &aws.Config{Region: &region})
	_, err := s3Client.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		return fmt.Errorf("unable to list S3 buckets - make sure you have valid admin AWS credentials: %w", err)
	}
	return nil
}

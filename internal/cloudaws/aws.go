package cloudaws

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"strings"
	"time"
)

const (
	// DefaultInstanceRegions is the default regions to use for spot instances
	DefaultInstanceRegions = "us-west-2,us-west-1,us-east-2"
)

type AWS struct {
	cfg          aws.Config
	name         string
	region       string
	emailAddress string
}

func New(name, region, emailAddress string) (*AWS, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load default aws config: %w", err)
	}

	if err := checkS3Access(cfg); err != nil {
		return nil, err
	}

	return &AWS{
		cfg:          cfg,
		name:         name,
		region:       region,
		emailAddress: emailAddress,
	}, nil
}

func (c *AWS) Setup(ctx context.Context) error {
	if err := c.s3Setup(ctx); err != nil {
		return err
	}
	if err := c.rolesSetup(ctx); err != nil {
		return err
	}
	return nil
}

func (c *AWS) SubscribeNotifications(ctx context.Context) error {
	return c.subscribeNotifications(ctx)
}

func (c *AWS) subscribeNotifications(ctx context.Context) error {
	snsClient := sns.NewFromConfig(c.cfg)
	resp, err := snsClient.ListTopics(ctx, &sns.ListTopicsInput{NextToken: aws.String("")})
	if err != nil {
		return fmt.Errorf("failed to list sns topics: %w", err)
	}

	for _, topic := range resp.Topics {
		if c.name == strings.Split(*topic.TopicArn, ":")[5] {
			resp, err := snsClient.ListSubscriptionsByTopic(ctx, &sns.ListSubscriptionsByTopicInput{
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
			_, err = snsClient.Subscribe(ctx, &sns.SubscribeInput{
				Protocol: aws.String("email"),
				TopicArn: aws.String(*topic.TopicArn),
				Endpoint: aws.String(c.emailAddress),
			})
			if err != nil {
				return fmt.Errorf("failed to setup email notifications: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("failed to subscribe to notifications - unable to find topic %v", c.name)
}

func (c *AWS) rolesSetup(ctx context.Context) error {
	iamClient := iam.NewFromConfig(c.cfg)

	_, err := iamClient.CreateServiceLinkedRole(ctx, &iam.CreateServiceLinkedRoleInput{
		AWSServiceName: aws.String("spot.amazonaws.com"),
	})
	if err != nil {
		var invalidInputException *iamtypes.InvalidInputException
		if errors.Is(err, invalidInputException) {
			return fmt.Errorf("failed to create spot.amazonaws.com service linked role: %w", err)
		}
	}

	_, err = iamClient.CreateServiceLinkedRole(ctx, &iam.CreateServiceLinkedRoleInput{
		AWSServiceName: aws.String("spotfleet.amazonaws.com"),
	})
	if err != nil {
		var invalidInputException *iamtypes.InvalidInputException
		if errors.Is(err, invalidInputException) {
			return fmt.Errorf("failed to create spotfleet.amazonaws.com service linked role: %w", err)
		}
	}

	return nil
}

func (c *AWS) s3Setup(ctx context.Context) error {
	s3Client := s3.NewFromConfig(c.cfg)
	_, err := s3Client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &c.name})
	if err != nil {
		var noSuchBucket *s3types.NoSuchBucket
		if !errors.As(err, &noSuchBucket) {
			return fmt.Errorf("unknown S3 error: %w", err)
		}

		bucketInput := &s3.CreateBucketInput{
			Bucket: &c.name,
		}
		if c.region != "us-east-1" {
			bucketInput.CreateBucketConfiguration = &s3types.CreateBucketConfiguration{
				LocationConstraint: s3types.BucketLocationConstraint(c.region),
			}
		}

		output, err := s3Client.CreateBucket(ctx, bucketInput)
		if err != nil {
			return fmt.Errorf("failed to create bucket %v - note that this bucket name must be globally unique: output:%v err:%w", c.name, output, err)
		}
	}
	return nil
}

func checkS3Access(cfg aws.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 10)
	defer cancel()

	s3Client := s3.NewFromConfig(cfg)
	_, err := s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return fmt.Errorf("unable to list S3 buckets - make sure you have valid admin AWS credentials: %w", err)
	}
	return nil
}

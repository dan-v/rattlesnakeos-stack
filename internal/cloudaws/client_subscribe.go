package cloudaws

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"strings"
	"time"
)

type SubscribeClient struct {
	cfg    aws.Config
	name   string
	region string
	email  string
}

func NewSubscribeClient(name, region, email string) (*SubscribeClient, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load default aws config: %w", err)
	}

	if err := checkSNSAccess(cfg); err != nil {
		return nil, err
	}

	return &SubscribeClient{
		cfg:    cfg,
		name:   name,
		region: region,
		email:  email,
	}, nil
}

func (c *SubscribeClient) Subscribe(ctx context.Context) error {
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
				if *subscription.Endpoint == c.email {
					return nil
				}
			}

			// subscribe if not setup
			_, err = snsClient.Subscribe(ctx, &sns.SubscribeInput{
				Protocol: aws.String("email"),
				TopicArn: aws.String(*topic.TopicArn),
				Endpoint: aws.String(c.email),
			})
			if err != nil {
				return fmt.Errorf("failed to setup email notifications: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("failed to subscribe to notifications - unable to find topic %v", c.name)
}

func checkSNSAccess(cfg aws.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 10)
	defer cancel()

	snsClient := sns.NewFromConfig(cfg)
	_, err := snsClient.ListTopics(ctx, &sns.ListTopicsInput{NextToken: aws.String("")})
	if err != nil {
		return fmt.Errorf("unable to list S3 buckets - make sure you have valid admin AWS credentials: %w", err)
	}
	return nil
}
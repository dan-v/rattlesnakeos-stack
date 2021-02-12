package stack

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sns"
	stackaws "github.com/dan-v/rattlesnakeos-stack/internal/aws"
	"github.com/dan-v/rattlesnakeos-stack/internal/devices"
	"github.com/dan-v/rattlesnakeos-stack/internal/terraform"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

const (
	DefaultTrustedRepoBase = "https://github.com/rattlesnakeos/"
	MinimumChromiumVersion = 86
)

const (
	awsErrCodeNoSuchBucket = "NoSuchBucket"
	awsErrCodeNotFound     = "NotFound"
	lambdaFunctionFilename = "lambda_spot_function.py"
	lambdaZipFilename      = "lambda_spot.zip"
	buildScriptFilename    = "build.sh"
)

type Config struct {
	Name                   string
	Region                 string
	Device                 string
	DeviceDetails          devices.Device
	Email                  string
	InstanceType           string
	InstanceRegions        string
	SkipPrice              string
	MaxPrice               string
	SSHKey                 string
	Version                string
	Schedule               string
	ChromiumVersion        string
	CustomPatches          *CustomPatches
	CustomScripts          *CustomScripts
	CustomPrebuilts        *CustomPrebuilts
	CustomManifestRemotes  *CustomManifestRemotes
	CustomManifestProjects *CustomManifestProjects
	HostsFile              string
	AMI                    string
}

type Stack struct {
	config          *Config
	terraformClient *terraform.Client
	terraformOutput string
}

func New(config *Config, buildScript, buildScriptTemplate, lambdaTemplate, terraformTemplate string) (*Stack, error) {
	err := checkAWSCreds(config.Region)
	if err != nil {
		return nil, err
	}

	err = s3BucketSetup(config.Name, config.Region)
	if err != nil {
		return nil, err
	}

	outputDirName := fmt.Sprintf("output_%v", config.Name)
	outputDirFullPath, err := filepath.Abs(outputDirName)
	if err != nil {
		return nil, err
	}
	buildScriptFilePath := filepath.Join(outputDirFullPath, buildScriptFilename)
	lambdaFunctionFilePath := filepath.Join(outputDirFullPath, lambdaFunctionFilename)
	lambdaZipFilePath := filepath.Join(outputDirFullPath, lambdaZipFilename)
	tfDirFilePath := filepath.Join(outputDirFullPath, "tf")
	err = os.MkdirAll(outputDirFullPath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	// render build script
	renderedBuildScriptTemplate, err := renderTemplate(buildScriptTemplate, config)
	if err != nil {
		return nil, fmt.Errorf("failed to render build script: %w", err)
	}

	// render lambda
	regionAMIs, _ := json.Marshal(stackaws.RegionAMIs)
	lambdaConfig := struct {
		Config     Config
		RegionAMIs string
	}{
		*config,
		string(regionAMIs),
	}
	renderedLambdaFunction, err := renderTemplate(lambdaTemplate, lambdaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to render lambda function: %w", err)
	}

	// render terraform
	terraformConfig := struct {
		Config                  Config
		LambdaZipFileLocation   string
		BuildScriptFileLocation string
	}{
		*config,
		lambdaZipFilePath,
		buildScriptFilePath,
	}
	renderedTerraform, err := renderTemplate(terraformTemplate, terraformConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to render terraform script: %w", err)
	}

	// write out shell script
	updatedBuildScript := strings.Replace(buildScript, "####REPLACE-VARS####", string(renderedBuildScriptTemplate), 1)
	err = ioutil.WriteFile(buildScriptFilePath, []byte(updatedBuildScript), 0644)
	if err != nil {
		return nil, err
	}

	// write out lambda function and zip it up
	err = ioutil.WriteFile(lambdaFunctionFilePath, renderedLambdaFunction, 0644)
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(lambdaFunctionFilePath, 0644); err != nil {
		return nil, err
	}

	err = zipFiles(lambdaZipFilePath, []string{lambdaFunctionFilePath})
	if err != nil {
		return nil, err
	}

	// write out terraform
	if err := os.MkdirAll(tfDirFilePath, 0777); err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(filepath.Join(tfDirFilePath, "main.tf"), renderedTerraform, 0777); err != nil {
		return nil, err
	}

	terraformClient, err := terraform.New(outputDirFullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create terraform client: %w", err)
	}

	return &Stack{
		config:          config,
		terraformClient: terraformClient,
	}, nil
}

func (s *Stack) Apply() error {
	sess, err := session.NewSession(aws.NewConfig().WithCredentialsChainVerboseErrors(true))
	if err != nil {
		return err
	}

	log.Info("Creating required service linked roles if needed")
	_, err = iam.New(sess).CreateServiceLinkedRole(&iam.CreateServiceLinkedRoleInput{
		AWSServiceName: aws.String("spot.amazonaws.com"),
	})
	if errWithCode, ok := err.(awserr.Error); ok && iam.ErrCodeInvalidInputException == errWithCode.Code() {
		log.Debug("spot service linked role already exists")
	}

	_, err = iam.New(sess).CreateServiceLinkedRole(&iam.CreateServiceLinkedRoleInput{
		AWSServiceName: aws.String("spotfleet.amazonaws.com"),
	})
	if errWithCode, ok := err.(awserr.Error); ok && iam.ErrCodeInvalidInputException == errWithCode.Code() {
		log.Debug("spotfleet service role already exists")
	}

	log.Info("Creating/updating AWS resources")
	tfDir := filepath.Join(s.terraformOutput, "tf")
	_, err = s.terraformClient.Init([]string{tfDir})
	if err != nil {
		return err
	}
	_, err = s.terraformClient.Apply([]string{"-auto-approve", tfDir})
	if err != nil {
		return err
	}
	log.Infof("Successfully deployed/updated AWS resources for stack %v", s.config.Name)

	snsClient := sns.New(sess, &aws.Config{Region: &s.config.Region})
	resp, err := snsClient.ListTopics(&sns.ListTopicsInput{NextToken: aws.String("")})
	for _, topic := range resp.Topics {
		topicName := strings.Split(*topic.TopicArn, ":")[5]
		if topicName == s.config.Name {
			// check if subscription exists
			resp, err := snsClient.ListSubscriptionsByTopic(&sns.ListSubscriptionsByTopicInput{
				NextToken: aws.String(""),
				TopicArn:  aws.String(*topic.TopicArn),
			})
			if err != nil {
				return fmt.Errorf("Failed to list SNS subscriptions for topic %v: %v", *topic.TopicArn, err)
			}
			for _, subscription := range resp.Subscriptions {
				if *subscription.Endpoint == s.config.Email {
					return nil
				}
			}

			// subscribe if not setup
			_, err = snsClient.Subscribe(&sns.SubscribeInput{
				Protocol: aws.String("email"),
				TopicArn: aws.String(*topic.TopicArn),
				Endpoint: aws.String(s.config.Email),
			})
			if err != nil {
				return fmt.Errorf("Failed to setup email notifications: %v", err)
			}
			log.Infof("Successfully setup email notifications for %v - you'll "+
				"need to click link in confirmation email to get notifications.", s.config.Email)
			break
		}
	}

	return nil
}

func (s *Stack) Destroy() error {
	log.Info("Destroying AWS resources")
	_, err := s.terraformClient.Destroy([]string{})
	if err != nil {
		return err
	}
	log.Info("Successfully removed AWS resources")
	return nil
}

func renderTemplate(templateStr string, params interface{}) ([]byte, error) {
	templ, err := template.New("template").Delims("<%", "%>").Parse(templateStr)
	if err != nil {
		return nil, err
	}

	buffer := new(bytes.Buffer)

	if err = templ.Execute(buffer, params); err != nil {
		return nil, err
	}

	outputBytes, err := ioutil.ReadAll(buffer)
	if err != nil {
		return nil, err
	}
	return outputBytes, nil
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

func zipFiles(filename string, files []string) error {
	newFile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer newFile.Close()

	zipWriter := zip.NewWriter(newFile)
	defer zipWriter.Close()

	// Add files to zip
	for _, file := range files {

		zipfile, err := os.Open(file)
		if err != nil {
			return err
		}
		defer zipfile.Close()

		info, err := zipfile.Stat()
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, zipfile)
		if err != nil {
			return err
		}
	}
	return nil
}

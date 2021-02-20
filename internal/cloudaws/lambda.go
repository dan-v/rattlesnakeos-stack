package cloudaws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
)

func ExecuteLambdaFunction(functionName, region string, payload []byte) (*lambda.InvokeOutput, error) {
	sess, err := getSession()
	if err != nil {
		return nil, fmt.Errorf("failed to get session for lambda function execution: %w", err)
	}

	lambdaClient := lambda.New(sess, &aws.Config{Region: &region})
	output, err := lambdaClient.Invoke(&lambda.InvokeInput{
		FunctionName:   aws.String(functionName),
		InvocationType: aws.String("RequestResponse"),
		Payload:        payload,
	})
	if err != nil {
		return output, fmt.Errorf("failed to execute lambda function: %w", err)
	}
	return output, nil
}

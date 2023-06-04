package conns

import (
	"context"
	"fmt"
	"net/http"

	aws_sdkv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/apigatewayv2"
	"github.com/aws/aws-sdk-go/service/s3"
)

// PartitionHostname returns a hostname with the provider domain suffix for the partition
// e.g. PREFIX.amazonaws.com
// The prefix should not contain a trailing period.
func (client *AWSClient) PartitionHostname(prefix string) string {
	return fmt.Sprintf("%s.%s", prefix, client.DNSSuffix)
}

// RegionalHostname returns a hostname with the provider domain suffix for the region and partition
// e.g. PREFIX.us-west-2.amazonaws.com
// The prefix should not contain a trailing period.
func (client *AWSClient) RegionalHostname(prefix string) string {
	return fmt.Sprintf("%s.%s.%s", prefix, client.Region, client.DNSSuffix)
}

func (client *AWSClient) S3ConnURICleaningDisabled() *s3.S3 {
	return client.s3ConnURICleaningDisabled
}

// SetHTTPClient sets the http.Client used for AWS API calls.
// To have effect it must be called before the AWS SDK v1 Session is created.
func (client *AWSClient) SetHTTPClient(httpClient *http.Client) {
	if client.Session == nil {
		client.httpClient = httpClient
	}
}

// HTTPClient returns the http.Client used for AWS API calls.
func (client *AWSClient) HTTPClient() *http.Client {
	return client.httpClient
}

// APIGatewayInvokeURL returns the Amazon API Gateway (REST APIs) invoke URL for the configured AWS Region.
// See https://docs.aws.amazon.com/apigateway/latest/developerguide/how-to-call-api.html.
func (client *AWSClient) APIGatewayInvokeURL(restAPIID, stageName string) string {
	return fmt.Sprintf("https://%s/%s", client.RegionalHostname(fmt.Sprintf("%s.execute-api", restAPIID)), stageName)
}

// APIGatewayV2InvokeURL returns the Amazon API Gateway v2 (WebSocket & HTTP APIs) invoke URL for the configured AWS Region.
// See https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-publish.html and
// https://docs.aws.amazon.com/apigateway/latest/developerguide/apigateway-set-up-websocket-deployment.html.
func (client *AWSClient) APIGatewayV2InvokeURL(protocolType, apiID, stageName string) string {
	if protocolType == apigatewayv2.ProtocolTypeWebsocket {
		return fmt.Sprintf("wss://%s/%s", client.RegionalHostname(fmt.Sprintf("%s.execute-api", apiID)), stageName)
	}

	if stageName == "$default" {
		return fmt.Sprintf("https://%s/", client.RegionalHostname(fmt.Sprintf("%s.execute-api", apiID)))
	}

	return fmt.Sprintf("https://%s/%s", client.RegionalHostname(fmt.Sprintf("%s.execute-api", apiID)), stageName)
}

// CloudFrontDistributionHostedZoneID returns the Route 53 hosted zone ID
// for Amazon CloudFront distributions in the configured AWS partition.
func (client *AWSClient) CloudFrontDistributionHostedZoneID() string {
	if client.Partition == endpoints.AwsCnPartitionID {
		return "Z3RFFRIM2A3IF5" // See https://docs.amazonaws.cn/en_us/aws/latest/userguide/route53.html
	}
	return "Z2FDTNDATAQYW2" // See https://docs.aws.amazon.com/Route53/latest/APIReference/API_AliasTarget.html#Route53-Type-AliasTarget-HostedZoneId
}

// DefaultKMSKeyPolicy returns the default policy for KMS keys in the configured AWS partition.
func (client *AWSClient) DefaultKMSKeyPolicy() string {
	return fmt.Sprintf(`
{
	"Id": "default",
	"Version": "2012-10-17",
	"Statement": [
		{
			"Sid": "Enable IAM User Permissions",
			"Effect": "Allow",
			"Principal": {
				"AWS": "arn:%[1]s:iam::%[2]s:root"
			},
			"Action": "kms:*",
			"Resource": "*"
		}
	]
}	
`, client.Partition, client.AccountID)
}

// GlobalAcceleratorHostedZoneID returns the Route 53 hosted zone ID
// for AWS Global Accelerator accelerators in the configured AWS partition.
func (client *AWSClient) GlobalAcceleratorHostedZoneID() string {
	return "Z2BJ6XQ5FK7U4H" // See https://docs.aws.amazon.com/general/latest/gr/global_accelerator.html#global_accelerator_region
}

// conn returns the AWS SDK for Go v1 API client for the specified service.
func conn[T any](ctx context.Context, c *AWSClient, servicePackageName string) (T, error) {
	sp, ok := c.ServicePackages[servicePackageName]
	if !ok {
		var zero T
		return zero, fmt.Errorf("unknown service package: %s", servicePackageName)
	}

	v, ok := sp.(interface {
		NewConn(context.Context, *session.Session) (T, error)
	})
	if !ok {
		var zero T
		return zero, fmt.Errorf("no AWS SDK v1 API client factory: %s", servicePackageName)
	}

	conn, err := v.NewConn(ctx, c.Session)
	if err != nil {
		var zero T
		return zero, err
	}

	if v, ok := sp.(interface {
		CustomizeConn(context.Context, T, *session.Session) error
	}); ok {
		err := v.CustomizeConn(ctx, conn, c.Session)
		if err != nil {
			var zero T
			return zero, err
		}
	}

	return conn, nil
}

// client returns the AWS SDK for Go v2 API client for the specified service.
func client[T any](ctx context.Context, c *AWSClient, servicePackageName string) (T, error) {
	sp, ok := c.ServicePackages[servicePackageName]
	if !ok {
		var zero T
		return zero, fmt.Errorf("unknown service package: %s", servicePackageName)
	}

	v, ok := sp.(interface {
		NewClient(context.Context, aws_sdkv2.Config) (T, error)
	})
	if !ok {
		var zero T
		return zero, fmt.Errorf("no AWS SDK v2 API client factory: %s", servicePackageName)
	}

	// TODO: Add Cfg to AWSClient.
	client, err := v.NewClient(ctx, aws_sdkv2.Config{} /*c.Config*/)
	if err != nil {
		var zero T
		return zero, err
	}

	if v, ok := sp.(interface {
		CustomizeClient(context.Context, T, aws_sdkv2.Config) error
	}); ok {
		err := v.CustomizeClient(ctx, client, aws_sdkv2.Config{} /*c.Config*/)
		if err != nil {
			var zero T
			return zero, err
		}
	}

	return client, nil
}

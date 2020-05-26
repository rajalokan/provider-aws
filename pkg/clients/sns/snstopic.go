package sns

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sns"

	awsclients "github.com/crossplane/provider-aws/pkg/clients"
)

// TopicClient is the external client used for SNSTopic custom resource
type TopicClient interface {
	CreateTopicRequest(*sns.CreateTopicInput) sns.CreateTopicRequest
	ListTopicsRequest(*sns.ListTopicsInput) sns.ListTopicsRequest
	DeleteTopicRequest(*sns.DeleteTopicInput) sns.DeleteTopicRequest
}

// NewTopicClient returns a new client using AWS credentials as JSON encoded data.
func NewTopicClient(ctx context.Context, credentials []byte, region string, auth awsclients.AuthMethod) (TopicClient, error) {
	cfg, err := auth(ctx, credentials, awsclients.DefaultSection, region)
	if cfg == nil {
		return nil, err
	}
	return sns.New(*cfg), nil
}

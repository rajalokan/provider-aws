package sns

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sns"

	awsclients "github.com/crossplane/provider-aws/pkg/clients"
)

// SubscriptionClient is the external
type SubscriptionClient interface {
	SubscribeRequest(*sns.SubscribeInput) sns.SubscribeRequest
	ConfirmSubscriptionRequest(*sns.ConfirmSubscriptionInput) sns.ConfirmSubscriptionRequest
	UnsubscribeRequest(*sns.UnsubscribeInput) sns.UnsubscribeRequest
}

// NewSubscriptionClient returns a new client
func NewSubscriptionClient(ctx context.Context, crendentials []byte, region string, auth awsclients.AuthMethod) (SubscriptionClient, error) {
	cfg, err := auth(ctx, crendentials, awsclients.DefaultSection, region)
	if err != nil {
		return nil, err
	}
	return sns.New(*cfg), nil
}

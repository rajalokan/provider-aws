package sns

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/provider-aws/apis/notification/v1alpha1"
	aws "github.com/crossplane/provider-aws/pkg/clients"
	awsclients "github.com/crossplane/provider-aws/pkg/clients"
)

const (
	//SNSSubscriptionNotFound is the error code that is returned if SNS Subscription is not present
	SNSSubscriptionNotFound = "InvalidSNSSubscription.NotFound"

	// SNSSubscriptionInPendingConfirmation will be raise when SNS Subscription is found
	// but in "pending confirmation" state.
	SNSSubscriptionInPendingConfirmation = "InvalidSNSSubscription.PendingConfirmation"

	msgPendingConfirmation = "pending confirmation"
)

// SubscriptionNotFound will be raised when there is no SNSTopic
type SubscriptionNotFound struct{}

func (err *SubscriptionNotFound) Error() string {
	return fmt.Sprint(SNSSubscriptionNotFound)
}

// SubscriptionInPendingConfirmation will be raised when subscription is in
// "pending confirmation" state.
type SubscriptionInPendingConfirmation struct{}

func (err *SubscriptionInPendingConfirmation) Error() string {
	return fmt.Sprint(SNSSubscriptionInPendingConfirmation)
}

// SubscriptionClient is the external
type SubscriptionClient interface {
	ListSubscriptionsRequest(*sns.ListSubscriptionsInput) sns.ListSubscriptionsRequest
	ListSubscriptionsByTopicRequest(*sns.ListSubscriptionsByTopicInput) sns.ListSubscriptionsByTopicRequest
	SubscribeRequest(*sns.SubscribeInput) sns.SubscribeRequest
	ConfirmSubscriptionRequest(*sns.ConfirmSubscriptionInput) sns.ConfirmSubscriptionRequest
	UnsubscribeRequest(*sns.UnsubscribeInput) sns.UnsubscribeRequest
	GetSubscriptionAttributesRequest(*sns.GetSubscriptionAttributesInput) sns.GetSubscriptionAttributesRequest
}

// NewSubscriptionClient returns a new client
func NewSubscriptionClient(ctx context.Context, crendentials []byte, region string, auth awsclients.AuthMethod) (SubscriptionClient, error) {
	cfg, err := auth(ctx, crendentials, awsclients.DefaultSection, region)
	if err != nil {
		return nil, err
	}
	return sns.New(*cfg), nil
}

// GenerateSubscribeInput prepares input for SubscribeRequest
func GenerateSubscribeInput(p *v1alpha1.SNSSubscriptionParameters) *sns.SubscribeInput {
	input := &sns.SubscribeInput{
		Endpoint: p.Endpoint,
		Protocol: p.Protocol,
		TopicArn: aws.String(p.TopicArn),
	}

	return input
}

// GetSNSSubscription returns SNSSubscription if present or NotFound err
func GetSNSSubscription(ctx context.Context, c SubscriptionClient, cr *v1alpha1.SNSSubscription) (sns.Subscription, error) {
	// req := c.ListSubscriptionsRequest(&sns.ListSubscriptionsInput{})
	req := c.ListSubscriptionsByTopicRequest(&sns.ListSubscriptionsByTopicInput{
		TopicArn: aws.String(cr.Spec.ForProvider.TopicArn),
	})
	res, err := req.Send(ctx)
	if err != nil {
		return sns.Subscription{}, err
	}

	p := cr.Spec.ForProvider
	for _, sub := range res.Subscriptions {
		if cmp.Equal(sub.Endpoint, p.Endpoint) && cmp.Equal(sub.Protocol, p.Protocol) {
			return sub, nil
		}
	}

	return sns.Subscription{}, &SubscriptionNotFound{}

}

func getSubscriptionAttributes(p v1alpha1.SNSSubscriptionParameters) map[string]string {
	attr := make(map[string]string)
	attr["Owner"] = aws.StringValue(p.Endpoint)

	return attr
}

func getSubAttributes(attr map[string]string) map[string]string {
	newAttr := make(map[string]string)
	newAttr["Owner"] = attr["Owner"]

	return newAttr
}

// IsSNSSubscriptionUpToDate checks if object is up to date
func IsSNSSubscriptionUpToDate(p v1alpha1.SNSSubscriptionParameters, attr map[string]string) (bool, error) {
	pAttrs := getSubscriptionAttributes(p)
	isUpToDate := cmp.Equal(pAttrs, getSubAttributes(attr))
	return isUpToDate, nil
}

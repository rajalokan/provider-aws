/*
Copyright 2019 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sns

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/provider-aws/apis/notification/v1alpha1"
	awsclients "github.com/crossplane/provider-aws/pkg/clients"
)

const (
	//SNSSubscriptionNotFound is the error code that is returned if SNS Subscription is not present
	SNSSubscriptionNotFound = "InvalidSNSSubscription.NotFound"

	// SNSSubscriptionInPendingConfirmation will be raise when SNS Subscription is found
	// but in "pending confirmation" state.
	SNSSubscriptionInPendingConfirmation = "InvalidSNSSubscription.PendingConfirmation"

	msgPendingConfirmation = "pending confirmation"

	statusConfirmed           = "Confirmed"
	statusPendingConfirmation = "Pending Confirmation"
)

// IsErrorSubscriptionNotFound returns true if the error code indicates that the item was not found
func IsErrorSubscriptionNotFound(err error) bool {
	if _, ok := err.(*SubscriptionNotFound); ok {
		return true
	}
	return false
}

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
	ListSubscriptionsByTopicRequest(*sns.ListSubscriptionsByTopicInput) sns.ListSubscriptionsByTopicRequest
	SubscribeRequest(*sns.SubscribeInput) sns.SubscribeRequest
	ConfirmSubscriptionRequest(*sns.ConfirmSubscriptionInput) sns.ConfirmSubscriptionRequest
	UnsubscribeRequest(*sns.UnsubscribeInput) sns.UnsubscribeRequest
	GetSubscriptionAttributesRequest(*sns.GetSubscriptionAttributesInput) sns.GetSubscriptionAttributesRequest
	SetSubscriptionAttributesRequest(*sns.SetSubscriptionAttributesInput) sns.SetSubscriptionAttributesRequest
}

// NewSubscriptionClient returns a new client using AWS credentials as JSON encoded
// data
func NewSubscriptionClient(conf *aws.Config) (SubscriptionClient, error) {
	return sns.New(*conf), nil
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

// GenerateSubscriptionObservation is used to produce SNSSubscriptionObservation
// from resource at cloud & its attributes
func GenerateSubscriptionObservation(attr map[string]string) v1alpha1.SNSSubscriptionObservation {
	o := &v1alpha1.SNSSubscriptionObservation{}

	o.Arn = aws.String(attr["SubscriptionArn"])
	o.Owner = aws.String(attr["Owner"])
	if s, err := strconv.ParseBool(attr["PendingConfirmation"]); err == nil {
		if s == true {
			o.Status = aws.String("Pending Confirmation")
		} else {
			o.Status = aws.String("Confirmed")
		}
	}

	return *o
}

// LateInitializeSubscription fills the empty fields in
// *v1alpha1.SNSSubscriptionParameters with the values seen in
// sns.Subscription
func LateInitializeSubscription(in *v1alpha1.SNSSubscriptionParameters, sub sns.Subscription, attrs map[string]string) {
	in.Endpoint = awsclients.LateInitializeStringPtr(in.Endpoint, sub.Endpoint)
	in.Protocol = awsclients.LateInitializeStringPtr(in.Protocol, sub.Protocol)
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

func getSubAttributes(p v1alpha1.SNSSubscriptionParameters) map[string]string {
	attr := make(map[string]string)
	// attr["Owner"] = aws.StringValue(p.Endpoint)

	return attr
}

// GetChangedSubAttributes will return the changed attributes  for a subscription
// in provider side
func GetChangedSubAttributes(p v1alpha1.SNSSubscriptionParameters, attrs map[string]string) map[string]string {
	subAttrs := getSubAttributes(p)
	changedAttrs := make(map[string]string)
	for k, v := range subAttrs {
		if v != attrs[k] {
			changedAttrs[k] = v
		}
	}

	return changedAttrs
}

// IsSNSSubscriptionUpToDate checks if object is up to date
func IsSNSSubscriptionUpToDate(p v1alpha1.SNSSubscriptionParameters, attr map[string]string) bool {
	return len(GetChangedSubAttributes(p, attr)) == 0
}

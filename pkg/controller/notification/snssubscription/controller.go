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

package snssubscription

import (
	"context"
	"reflect"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsarn "github.com/aws/aws-sdk-go-v2/aws/arn"
	awssns "github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/provider-aws/apis/notification/v1alpha1"
	"github.com/crossplane/provider-aws/pkg/clients/sns"
	snsclient "github.com/crossplane/provider-aws/pkg/clients/sns"
	"github.com/crossplane/provider-aws/pkg/controller/utils"
)

const (
	errNotSNSSubscription = "managed resource is not an SNSSubscription custom resource"

	errKubeSubscriptionUpdateFailed = "cannot update SNSSubscription custom resource"

	errCreateSubscriptionClient = "cannot create SNS Subscription client"
	errGetProvider              = "cannot get provider"
	errGetProviderSecret        = "cannot get provider secret"

	errClient = "cannot create a new SNSSubscription client"

	errUnexpectedObject    = "The managed resource is not a SNSSubscription resource"
	errList                = "failed to list SNS Subscription"
	errGetSubscription     = "failed to get SNS Subscription"
	errGetSubscriptionAttr = "failed to get SNS Subscription Attribute"
	errCreate              = "failed to create the SNS Subscription"
	errDelete              = "failed to delete the SNS Subscription"
	errUpdate              = "failed to update the SNS Subscription"
	errUpToDateFailed      = "cannot check whether object is up-to-date"
	errPendingConfirmation = "Subscription in Pending Confirmation state."

	pendingConfirmation    = "pending confirmation"
	pendingConfirmationArn = "PendingConfirmation"
)

// SetupSubscription adds a controller than reconciles SNSSubscription
func SetupSubscription(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1alpha1.SNSSubscriptionGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.SNSSubscription{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1alpha1.SNSSubscriptionGroupVersionKind),
			managed.WithExternalConnecter(&connector{
				kube:        mgr.GetClient(),
				newClientFn: sns.NewSubscriptionClient,
				awsConfigFn: utils.RetrieveAwsConfigFromProvider,
			}),
			managed.WithReferenceResolver(managed.NewAPISimpleReferenceResolver(mgr.GetClient())),
			managed.WithInitializers(),
			managed.WithConnectionPublishers(),
			managed.WithLogger(l.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
}

type connector struct {
	kube        client.Client
	newClientFn func(*aws.Config) (sns.SubscriptionClient, error)
	awsConfigFn func(context.Context, client.Reader, *corev1.ObjectReference) (*aws.Config, error)
}

func (conn *connector) Connect(ctx context.Context, mgd resource.Managed) (managed.ExternalClient, error) {

	cr, ok := mgd.(*v1alpha1.SNSSubscription)
	if !ok {
		return nil, errors.New(errUnexpectedObject)
	}

	awsconfig, err := conn.awsConfigFn(ctx, conn.kube, cr.Spec.ProviderReference)
	if err != nil {
		return nil, err
	}

	c, err := conn.newClientFn(awsconfig)
	if err != nil {
		return nil, errors.Wrap(err, errClient)
	}
	return &external{c, conn.kube}, nil
}

type external struct {
	client snsclient.SubscriptionClient
	kube   client.Client
}

func (e *external) Observe(ctx context.Context, mgd resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mgd.(*v1alpha1.SNSSubscription)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errUnexpectedObject)
	}

	sub, err := snsclient.GetSNSSubscription(ctx, e.client, cr)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(resource.Ignore(sns.IsErrorSubscriptionNotFound, err), errGetSubscription)
	}

	// If subscription retrieved is in PendingConfirmation state, then
	// do nothing.
	if aws.StringValue(sub.SubscriptionArn) == pendingConfirmationArn {
		// Set confirmation status, so it is visible at kubectl get.
		cr.Status.AtProvider.Status = aws.String("Pending Confirmation")
		return managed.ExternalObservation{}, errors.New(errPendingConfirmation)
	}

	subArn := meta.GetExternalName(cr)
	// Watch when subscription is confirmed by user. We need to set external
	// name to proper arn at provider & update this in db.
	// This will be a one time action.
	if awsarn.IsARN(*sub.SubscriptionArn) && !awsarn.IsARN(subArn) {
		meta.SetExternalName(cr, aws.StringValue(sub.SubscriptionArn))
		if err := e.kube.Update(ctx, cr); err != nil {
			return managed.ExternalObservation{}, errors.Wrap(err, "Failure during kube update")
		}
	}

	res, err := e.client.GetSubscriptionAttributesRequest(&awssns.GetSubscriptionAttributesInput{
		SubscriptionArn: aws.String(*sub.SubscriptionArn),
	}).Send(ctx)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetSubscriptionAttr)
	}

	current := cr.Spec.ForProvider.DeepCopy()
	snsclient.LateInitializeSubscription(&cr.Spec.ForProvider, sub, res.Attributes)
	if !reflect.DeepEqual(current, &cr.Spec.ForProvider) {
		if err := e.kube.Update(ctx, cr); err != nil {
			return managed.ExternalObservation{}, errors.Wrap(err, errKubeSubscriptionUpdateFailed)
		}
	}

	cr.SetConditions(runtimev1alpha1.Available())

	// GenerateObservation
	cr.Status.AtProvider = snsclient.GenerateSubscriptionObservation(res.Attributes)

	upToDate := snsclient.IsSNSSubscriptionUpToDate(cr.Spec.ForProvider, res.Attributes)

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil
}

func (e *external) Create(ctx context.Context, mgd resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mgd.(*v1alpha1.SNSSubscription)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errUnexpectedObject)
	}

	cr.Status.SetConditions(runtimev1alpha1.Creating())

	input := snsclient.GenerateSubscribeInput(&cr.Spec.ForProvider)
	res, err := e.client.SubscribeRequest(input).Send(ctx)

	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	meta.SetExternalName(cr, aws.StringValue(res.SubscribeOutput.SubscriptionArn))
	if err := e.kube.Update(ctx, cr); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "Failure during kube update")
	}

	return managed.ExternalCreation{}, errors.Wrap(nil, errCreate)
}

func (e *external) Update(ctx context.Context, mgd resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mgd.(*v1alpha1.SNSSubscription)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errUnexpectedObject)
	}

	// Fetch Subscription Attributes again
	resp, err := e.client.GetSubscriptionAttributesRequest(&awssns.GetSubscriptionAttributesInput{
		SubscriptionArn: aws.String(meta.GetExternalName(cr)),
	}).Send(ctx)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
	}

	// Update Subscription
	attrs := snsclient.GetChangedSubAttributes(cr.Spec.ForProvider, resp.Attributes)
	for k, v := range attrs {
		_, err := e.client.SetSubscriptionAttributesRequest(&awssns.SetSubscriptionAttributesInput{
			AttributeName:   aws.String(k),
			AttributeValue:  aws.String(v),
			SubscriptionArn: aws.String(meta.GetExternalName(cr)),
		}).Send(ctx)
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
		}
	}

	return managed.ExternalUpdate{}, errors.Wrap(nil, errUpdate)
}

func (e *external) Delete(ctx context.Context, mgd resource.Managed) error {
	cr, ok := mgd.(*v1alpha1.SNSSubscription)
	if !ok {
		return errors.New(errUnexpectedObject)
	}

	if !awsarn.IsARN(meta.GetExternalName(cr)) {
		return errors.New("Can't delete a subscription in PendingConfirmation state")
	}

	_, err := e.client.UnsubscribeRequest(&awssns.UnsubscribeInput{
		SubscriptionArn: aws.String(meta.GetExternalName(cr)),
	}).Send(ctx)

	return errors.Wrap(err, errDelete)
}

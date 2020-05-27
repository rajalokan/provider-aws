package snssubscription

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsarn "github.com/aws/aws-sdk-go-v2/aws/arn"
	awssns "github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/provider-aws/apis/notification/v1alpha1"
	awsv1alpha3 "github.com/crossplane/provider-aws/apis/v1alpha3"
	awsclients "github.com/crossplane/provider-aws/pkg/clients"
	snsclient "github.com/crossplane/provider-aws/pkg/clients/sns"
)

const (
	errNotSNSSubscription = "managed resource is not an SNSSubscription custom resource"

	errCreateSubscriptionClient = "cannot create SNS Subscription client"
	errGetProvider              = "cannot get provider"
	errGetProviderSecret        = "cannot get provider secret"

	errUnexpectedObject = "The managed resource is not a SNSSubscription resource"
	errList             = "failed to list SNS Subscription"
	errCreate           = "failed to create the SNS Subscription"
	errDelete           = "failed to delete the SNS Subscription"
	errUpdate           = "failed to update the SNS Subscription"
)

// SetupSubscription adds a controller than reconciles SNSSubscription
func SetupSubscription(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1alpha1.SNSSubscriptionGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.SNSSubscription{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1alpha1.SNSSubscriptionGroupVersionKind),
			managed.WithExternalConnecter(&connector{kube: mgr.GetClient(), newClientFn: snsclient.NewSubscriptionClient}),
			managed.WithReferenceResolver(managed.NewAPISimpleReferenceResolver(mgr.GetClient())),
			managed.WithConnectionPublishers(),
			managed.WithLogger(l.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
}

type connector struct {
	kube        client.Client
	newClientFn func(ctx context.Context, credentials []byte, region string, auth awsclients.AuthMethod) (snsclient.SubscriptionClient, error)
}

func (c *connector) Connect(ctx context.Context, mgd resource.Managed) (managed.ExternalClient, error) {
	fmt.Println("Connect")
	cr, ok := mgd.(*v1alpha1.SNSSubscription)
	if !ok {
		return nil, errors.New(errNotSNSSubscription)
	}

	p := &awsv1alpha3.Provider{}

	if err := c.kube.Get(ctx, meta.NamespacedNameOf(cr.Spec.ProviderReference), p); err != nil {
		return nil, errors.Wrap(err, errGetProvider)
	}

	if aws.BoolValue(p.Spec.UseServiceAccount) {
		policyClient, err := c.newClientFn(ctx, []byte{}, p.Spec.Region, awsclients.UsePodServiceAccount)
		return &external{client: policyClient, kube: c.kube}, errors.Wrap(err, errCreateSubscriptionClient)
	}

	if p.GetCredentialsSecretReference() == nil {
		return nil, errors.New(errGetProviderSecret)
	}

	s := &corev1.Secret{}
	n := types.NamespacedName{Namespace: p.Spec.CredentialsSecretRef.Namespace, Name: p.Spec.CredentialsSecretRef.Name}

	if err := c.kube.Get(ctx, n, s); err != nil {
		return nil, errors.Wrap(err, errGetProviderSecret)
	}

	topicClient, err := c.newClientFn(ctx, s.Data[p.Spec.CredentialsSecretRef.Key], p.Spec.Region, awsclients.UseProviderSecret)
	return &external{client: topicClient, kube: c.kube}, errors.Wrap(err, errCreateSubscriptionClient)

}

type external struct {
	client snsclient.SubscriptionClient
	kube   client.Client
}

func (e *external) Observe(ctx context.Context, mgd resource.Managed) (managed.ExternalObservation, error) {
	fmt.Println("In Observe")
	cr, ok := mgd.(*v1alpha1.SNSSubscription)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errUnexpectedObject)
	}

	if !awsarn.IsARN(meta.GetExternalName(cr)) {
		return managed.ExternalObservation{}, nil
	}

	resp, err := e.client.ConfirmSubscriptionRequest(&awssns.ConfirmSubscriptionInput{}).Send(ctx)
	if err != nil {
		return managed.ExternalObservation{}, errors.New(errList)
	}

	if resp.SubscriptionArn == nil {
		return managed.ExternalObservation{}, errors.New(errList)
	}
	cr.SetConditions(runtimev1alpha1.Available())

	cr.Status.AtProvider = v1alpha1.SNSSubscriptionObservation{
		Arn: aws.String(*resp.SubscriptionArn),
	}

	update := true

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: update,
	}, nil
}

func (e *external) Create(ctx context.Context, mgd resource.Managed) (managed.ExternalCreation, error) {
	fmt.Println("In Create")
	cr, ok := mgd.(*v1alpha1.SNSSubscription)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errUnexpectedObject)
	}
	cr.Status.SetConditions(runtimev1alpha1.Creating())

	createResp, err := e.client.SubscribeRequest(&awssns.SubscribeInput{}).Send(ctx)

	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	meta.SetExternalName(cr, aws.StringValue(createResp.SubscriptionArn))
	return managed.ExternalCreation{}, errors.Wrap(e.kube.Update(ctx, cr), errCreate)
}

func (e *external) Update(ctx context.Context, mgd resource.Managed) (managed.ExternalUpdate, error) {
	fmt.Println("In Update")
	_, ok := mgd.(*v1alpha1.SNSSubscription)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errUnexpectedObject)
	}
	// Update Subscription
	return managed.ExternalUpdate{}, errors.Wrap(nil, errUpdate)
}

func (e *external) Delete(ctx context.Context, mgd resource.Managed) error {
	fmt.Println("In Delete")
	_, ok := mgd.(*v1alpha1.SNSSubscription)
	if !ok {
		return errors.New(errUnexpectedObject)
	}
	_, err := e.client.UnsubscribeRequest(&awssns.UnsubscribeInput{}).Send(ctx)
	return errors.Wrap(err, errDelete)
}

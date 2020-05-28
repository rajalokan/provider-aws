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
	fmt.Println("\n\n\nIn Observe -- subscription")
	cr, ok := mgd.(*v1alpha1.SNSSubscription)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errUnexpectedObject)
	}

	fmt.Println("External Name : " + meta.GetExternalName(cr))

	externalName := meta.GetExternalName(cr)

	sub, err := snsclient.GetSNSSubscription(ctx, e.client, cr)
	if _, ok := err.(*snsclient.SubscriptionNotFound); ok {
		fmt.Println("Subscription not found.")
		fmt.Println("")
		fmt.Println("")
		fmt.Println("")
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	if aws.StringValue(sub.SubscriptionArn) == pendingConfirmationArn {
		fmt.Println("Subscription in pending confirmation. Skipping it.")
		fmt.Println("")
		fmt.Println("")
		fmt.Println("")
		return managed.ExternalObservation{
			ResourceExists: true,
		}, nil
	}

	if awsarn.IsARN(*sub.SubscriptionArn) && !awsarn.IsARN(externalName) {
		meta.SetExternalName(cr, aws.StringValue(sub.SubscriptionArn))
		if err := e.kube.Update(ctx, cr); err != nil {
			return managed.ExternalObservation{}, errors.Wrap(err, "Failure during kube update")
		}
	}

	cr.SetConditions(runtimev1alpha1.Available())

	res, err := e.client.GetSubscriptionAttributesRequest(&awssns.GetSubscriptionAttributesInput{
		SubscriptionArn: aws.String(*sub.SubscriptionArn),
	}).Send(ctx)
	fmt.Println("Attributes are")
	fmt.Println(res)

	upToDate, err := snsclient.IsSNSSubscriptionUpToDate(cr.Spec.ForProvider, res.Attributes)

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil
}

func (e *external) Create(ctx context.Context, mgd resource.Managed) (managed.ExternalCreation, error) {
	fmt.Println("\n\n\nIn Create - Subscription")

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

	cr.Status.AtProvider.Arn = res.SubscribeOutput.SubscriptionArn

	return managed.ExternalCreation{}, errors.Wrap(nil, errCreate)
}

func (e *external) Update(ctx context.Context, mgd resource.Managed) (managed.ExternalUpdate, error) {
	fmt.Println("\n\n\nIn Update - Subscription")
	_, ok := mgd.(*v1alpha1.SNSSubscription)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errUnexpectedObject)
	}
	// Update Subscription
	return managed.ExternalUpdate{}, errors.Wrap(nil, errUpdate)
}

func (e *external) Delete(ctx context.Context, mgd resource.Managed) error {
	fmt.Println("\n\n\nIn Delete - subscription")
	cr, ok := mgd.(*v1alpha1.SNSSubscription)
	if !ok {
		return errors.New(errUnexpectedObject)
	}
	_, err := e.client.UnsubscribeRequest(&awssns.UnsubscribeInput{
		SubscriptionArn: aws.String(meta.GetExternalName(cr)),
	}).Send(ctx)

	return errors.Wrap(err, errDelete)
}

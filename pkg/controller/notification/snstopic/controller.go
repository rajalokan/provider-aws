package snstopic

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
	errNotSNSTopic = "managed resource is not an SNSTopic custom resource"

	errCreateTopicClient = "cannot create SNS Topic client"
	errGetProvider       = "cannot get provider"
	errGetProviderSecret = "cannot get provider secret"

	errUnexpectedObject = "The managed resource is not a SNSTopic resource"
	errList             = "failed to list SNS Topics"
	errCreate           = "failed to create the SNS Topic"
	errDelete           = "failed to delete the SNS Topic"
	errUpdate           = "failed to update the SNS Topic"

	errNoTopics = "No listed topics"
)

// SetupSNSTopic adds a controller that reconciles SNSTopic.
func SetupSNSTopic(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1alpha1.SNSTopicGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.SNSTopic{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1alpha1.SNSTopicGroupVersionKind),
			managed.WithExternalConnecter(&connector{kube: mgr.GetClient(), newClientFn: snsclient.NewTopicClient}),
			managed.WithReferenceResolver(managed.NewAPISimpleReferenceResolver(mgr.GetClient())),
			managed.WithInitializers(),
			managed.WithConnectionPublishers(),
			managed.WithLogger(l.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
}

type connector struct {
	kube        client.Client
	newClientFn func(ctx context.Context, credentials []byte, region string, auth awsclients.AuthMethod) (snsclient.TopicClient, error)
}

func (c *connector) Connect(ctx context.Context, mgd resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mgd.(*v1alpha1.SNSTopic)
	if !ok {
		return nil, errors.New(errNotSNSTopic)
	}

	p := &awsv1alpha3.Provider{}

	if err := c.kube.Get(ctx, meta.NamespacedNameOf(cr.Spec.ProviderReference), p); err != nil {
		return nil, errors.Wrap(err, errGetProvider)
	}

	if aws.BoolValue(p.Spec.UseServiceAccount) {
		policyClient, err := c.newClientFn(ctx, []byte{}, p.Spec.Region, awsclients.UsePodServiceAccount)
		return &external{client: policyClient, kube: c.kube}, errors.Wrap(err, errCreateTopicClient)
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
	return &external{client: topicClient, kube: c.kube}, errors.Wrap(err, errCreateTopicClient)
}

type external struct {
	client snsclient.TopicClient
	kube   client.Client
}

func (e *external) Observe(ctx context.Context, mgd resource.Managed) (managed.ExternalObservation, error) {
	fmt.Println("\n\n\nIn Observe")
	cr, ok := mgd.(*v1alpha1.SNSTopic)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errUnexpectedObject)
	}
	fmt.Println("External Name : " + meta.GetExternalName(cr))

	if !awsarn.IsARN(meta.GetExternalName(cr)) {
		fmt.Println("External Name doesn't exit. Means Resource doesn't exist. Going to create flow")
		return managed.ExternalObservation{}, nil
	}

	topic, err := snsclient.GetSNSTopic(ctx, e.client, meta.GetExternalName(cr))

	if _, ok := err.(*snsclient.NotFound); ok {
		fmt.Println("Topic not found. Going to create flow")
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	if err != nil {
		// Either there is err and retry. Or Resource does not exist.
		fmt.Println("Errored during getting topic. Retry")
		return managed.ExternalObservation{
			ResourceExists:    false,
			ConnectionDetails: managed.ConnectionDetails{},
		}, errors.Wrap(err, errList)
	}

	fmt.Println("Topic is present")
	fmt.Println(topic)

	cr.SetConditions(runtimev1alpha1.Available())

	// cr.Status.AtProvider = v1alpha1.SNSTopicObservation{
	// 	Arn: aws.String(*topic.TopicArn),
	// }

	// update, err := snsclient.IsTopicUpToDate(cr.Spec.ForProvider, *versionRsp.PolicyVersion)
	// if err != nil {
	// 	return managed.ExternalObservation{}, errors.Wrap(err, errUpToDate)
	// }
	topicArn := meta.GetExternalName(cr)
	resp, err := e.client.GetTopicAttributesRequest(&awssns.GetTopicAttributesInput{
		TopicArn: aws.String(topicArn),
	}).Send(ctx)

	fmt.Println("Attributes are")
	fmt.Println(resp.Attributes)

	upToDate, err := snsclient.IsSNSTopicUpToDate(cr.Spec.ForProvider, resp.Attributes)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDelete)
	}
	fmt.Println(upToDate)

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil

}

func (e *external) Create(ctx context.Context, mgd resource.Managed) (managed.ExternalCreation, error) {
	fmt.Println("In Create")
	cr, ok := mgd.(*v1alpha1.SNSTopic)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errUnexpectedObject)
	}

	cr.Status.SetConditions(runtimev1alpha1.Creating())

	input := snsclient.GenerateCreateTopicInput(&cr.Spec.ForProvider)
	resp, err := e.client.CreateTopicRequest(input).Send(ctx)

	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	meta.SetExternalName(cr, aws.StringValue(resp.CreateTopicOutput.TopicArn))
	if err := e.kube.Update(ctx, cr); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "Failure during kube update")
	}

	cr.Status.AtProvider.Arn = resp.CreateTopicOutput.TopicArn

	return managed.ExternalCreation{}, errors.Wrap(nil, errCreate)
}

func (e *external) Update(ctx context.Context, mgd resource.Managed) (managed.ExternalUpdate, error) {
	fmt.Println("In Update")
	_, ok := mgd.(*v1alpha1.SNSTopic)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errUnexpectedObject)
	}
	// Update Topic
	return managed.ExternalUpdate{}, errors.Wrap(nil, errUpdate)
}

func (e *external) Delete(ctx context.Context, mgd resource.Managed) error {
	fmt.Println("In Delete")

	cr, ok := mgd.(*v1alpha1.SNSTopic)
	if !ok {
		return errors.New(errUnexpectedObject)
	}
	_, err := e.client.DeleteTopicRequest(&awssns.DeleteTopicInput{
		TopicArn: aws.String(meta.GetExternalName(cr)),
	}).Send(ctx)
	return errors.Wrap(err, errDelete)
}

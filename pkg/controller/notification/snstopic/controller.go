package snstopic

import (
	"context"
	"fmt"

	awssns "github.com/aws/aws-sdk-go-v2/service/sns"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/provider-aws/apis/notification/v1alpha1"
	awsclients "github.com/crossplane/provider-aws/pkg/clients"
	snsclient "github.com/crossplane/provider-aws/pkg/clients/sns"
)

const (
	errNotSNSTopic = "managed resource is not an IAMPolicy custom resource"
	errGetProvider = "cannot get provider"
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
			managed.WithConnectionPublishers(),
			managed.WithLogger(l.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
}

type connector struct {
	kube        client.Client
	newClientFn func(ctx context.Context, credentials []byte, region string, auth awsclients.AuthMethod) (snsclient.TopicClient, error)
}

func (c *connector) Connect(ctx context.Context, mgd resource.Managed) (managed.ExternalClient, error) {
	return nil, nil
}

type external struct {
	client snsclient.TopicClient
	kube   client.Client
}

func (e *external) Observe(ctx context.Context, mgd resource.Managed) (managed.ExternalObservation, error) {
	fmt.Println("In Observe")
	fmt.Println(awssns.GetTopicAttributesInput{})
	return managed.ExternalObservation{}, nil
}

func (e *external) Create(ctx context.Context, mgd resource.Managed) (managed.ExternalCreation, error) {
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mgd resource.Managed) (managed.ExternalUpdate, error) {
	fmt.Println("In Update")
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mgd resource.Managed) error {
	fmt.Println("In Delete")
	return nil
}

package v1alpha1

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResolveReferences for SNS Topic managed type
func (mg *SNSSubscription) ResolveReferences(ctx context.Context, c client.Reader) error {
	r := reference.NewAPIResolver(c, mg)


	// Resolve spec.TopicID
	rsp, err := r.Resolve(ctx, reference.ResolutionRequest{
		CurrentValue: mg.Spec.ForProvider.TopicArn,
		Reference: mg.Spec.ForProvider.TopicArnRef,
		Selector: mg.Spec.ForProvider.TopicArnSelector,
		To: reference.To{Managed: &SNSTopic{}, List: &SNSTopicList{}},
		Extract: reference.ExternalName(),
	})

	if err != nil {
		return err
	}
	mg.Spec.ForProvider.TopicArn = rsp.ResolvedValue
	mg.Spec.ForProvider.TopicArnRef = rsp.ResolvedReference

	return nil
}
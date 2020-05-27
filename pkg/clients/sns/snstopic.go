package sns

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go/aws"

	"github.com/crossplane/provider-aws/apis/notification/v1alpha1"
	awsclients "github.com/crossplane/provider-aws/pkg/clients"
)

const (
	//SNSTopicNotFound is the error code that is returned if SNS Topic is not present
	SNSTopicNotFound = "InvalidSNSTopic.NotFound"
)

// NotFound will be raised when there is no SNSTopic
type NotFound struct{}

func (err *NotFound) Error() string {
	return fmt.Sprint(SNSTopicNotFound)
}

// TopicClient is the external client used for SNSTopic custom resource
type TopicClient interface {
	CreateTopicRequest(*sns.CreateTopicInput) sns.CreateTopicRequest
	ListTopicsRequest(*sns.ListTopicsInput) sns.ListTopicsRequest
	DeleteTopicRequest(*sns.DeleteTopicInput) sns.DeleteTopicRequest
	GetTopicAttributesRequest(*sns.GetTopicAttributesInput) sns.GetTopicAttributesRequest
}

// NewTopicClient returns a new client using AWS credentials as JSON encoded data.
func NewTopicClient(ctx context.Context, credentials []byte, region string, auth awsclients.AuthMethod) (TopicClient, error) {
	cfg, err := auth(ctx, credentials, awsclients.DefaultSection, region)
	if cfg == nil {
		return nil, err
	}
	return sns.New(*cfg), nil
}

// GetSNSTopic returns SNSTopic if present or NotFound err
func GetSNSTopic(ctx context.Context, c TopicClient, topicArn string) (sns.Topic, error) {
	req := c.ListTopicsRequest(&sns.ListTopicsInput{})
	res, err := req.Send(ctx)
	if err != nil {
		return sns.Topic{}, err
	}
	for _, topic := range res.Topics {
		if aws.StringValue(topic.TopicArn) == topicArn {
			return topic, nil
		}
	}
	return sns.Topic{}, nil
}

// GenerateCreateTopicInput prepares input for CreateTopicRequest
func GenerateCreateTopicInput(p *v1alpha1.SNSTopicParameters) *sns.CreateTopicInput {
	return &sns.CreateTopicInput{
		Name: p.Name,
	}
}

// // LateInitializeTopic fills the empty fields in *v1alpha1.SNSTopicParameters with the
// // values seen in sns.Topic.
// func LateInitializeTopic(in *v1alpha1.SNSTopicParameters, topic sns.Topic) {
// 	in.Name = awsclients.LateInitializeStringPtr(in.Name, topic.TopicArn)
// 	topic.GetTopicAttribute
// }

// func createPatch(in *sns.Topic, target *v1alpha1.SNSTopicParameters) (*v1alpha1.SNSTopicParameters, error) {
// 	currentParams := &v1alpha1.SNSTopicParameters{}
// 	LateInitalizeTopic(currentParams, in)
// }

// IsSNSTopicUpToDate checks if object is up to date
func IsSNSTopicUpToDate(p v1alpha1.SNSTopicParameters, attr map[string]string) (bool, error) {
	fmt.Println("IsSNSTopicUpToDate")
	return true, nil
}

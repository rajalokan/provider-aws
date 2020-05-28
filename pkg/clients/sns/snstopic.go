package sns

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/provider-aws/apis/notification/v1alpha1"
	awsclients "github.com/crossplane/provider-aws/pkg/clients"
)

const (
	//SNSTopicNotFound is the error code that is returned if SNS Topic is not present
	SNSTopicNotFound = "InvalidSNSTopic.NotFound"
)

// TopicNotFound will be raised when there is no SNSTopic
type TopicNotFound struct{}

func (err *TopicNotFound) Error() string {
	return fmt.Sprint(SNSTopicNotFound)
}

// TopicClient is the external client used for SNSTopic custom resource
type TopicClient interface {
	CreateTopicRequest(*sns.CreateTopicInput) sns.CreateTopicRequest
	ListTopicsRequest(*sns.ListTopicsInput) sns.ListTopicsRequest
	DeleteTopicRequest(*sns.DeleteTopicInput) sns.DeleteTopicRequest
	GetTopicAttributesRequest(*sns.GetTopicAttributesInput) sns.GetTopicAttributesRequest
	SetTopicAttributesRequest(*sns.SetTopicAttributesInput) sns.SetTopicAttributesRequest
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
	return sns.Topic{}, &TopicNotFound{}
}

// GenerateCreateTopicInput prepares input for CreateTopicRequest
func GenerateCreateTopicInput(p *v1alpha1.SNSTopicParameters) *sns.CreateTopicInput {
	input := &sns.CreateTopicInput{
		Name:       p.Name,
		Attributes: getTopicAttributes(*p),
	}

	if len(p.Tags) != 0 {
		input.Tags = make([]sns.Tag, len(p.Tags))
		for i, val := range p.Tags {
			input.Tags[i] = sns.Tag{
				Key:   aws.String(val.Key),
				Value: aws.String(val.Value),
			}
		}
	}

	return input
}

// LateInitializeTopic fills the empty fields in *v1alpha1.SNSTopicParameters with the
// values seen in sns.Topic.
func LateInitializeTopic(in *v1alpha1.SNSTopicParameters, topic sns.Topic, attrs map[string]string) {
	in.Name = awsclients.LateInitializeStringPtr(in.Name, topic.TopicArn)
	setTopicAttributes(in, attrs)
}

func setTopicAttributes(in *v1alpha1.SNSTopicParameters, attrs map[string]string) {
	in.DisplayName = awsclients.LateInitializeStringPtr(in.DisplayName, aws.String(attrs["DisplayName"]))
}

// GetChangedAttributes will set the changed attributes for a topic in AWS side.
//
// Currently AWS SDK allows to set Attribute Topics one at a time.
// Please see https://docs.aws.amazon.com/sns/latest/api/API_SetTopicAttributes.html
// So we need to compare each topic attribute and call SetTopicAttribute for ones which has
// changed.
func GetChangedAttributes(p v1alpha1.SNSTopicParameters, attrs map[string]string) map[string]string {
	topicAttrs := getTopicAttributes(p)
	correctAttrs := getCorrectAttributes(attrs)
	changedAttrs := make(map[string]string)
	for k, v := range topicAttrs {
		if v != correctAttrs[k] {
			fmt.Println("Updating key - ", k)
			changedAttrs[k] = v
		}
	}

	return changedAttrs
}

// func createPatch(in *sns.Topic, target *v1alpha1.SNSTopicParameters) (*v1alpha1.SNSTopicParameters, error) {
// 	currentParams := &v1alpha1.SNSTopicParameters{}
// 	LateInitalizeTopic(currentParams, in)
// }

func getTopicAttributes(p v1alpha1.SNSTopicParameters) map[string]string {
	attr := make(map[string]string)
	attr["DisplayName"] = aws.StringValue(p.DisplayName)

	return attr
}

func getCorrectAttributes(attr map[string]string) map[string]string {
	newAttr := make(map[string]string)
	newAttr["DisplayName"] = attr["DisplayName"]

	return newAttr
}

// IsSNSTopicUpToDate checks if object is up to date
func IsSNSTopicUpToDate(p v1alpha1.SNSTopicParameters, attr map[string]string) (bool, error) {
	pAttrs := getTopicAttributes(p)
	isUpToDate := cmp.Equal(pAttrs, getCorrectAttributes(attr))
	return isUpToDate, nil
}

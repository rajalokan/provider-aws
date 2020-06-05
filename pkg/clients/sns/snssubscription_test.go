package sns

import (
	"testing"

	awssns "github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/provider-aws/apis/notification/v1alpha1"
)

var (
	subName          = "some-subscription"
	subOwner         = "owner"
	subEmailProtocol = "email"
	subEmailEndpoint = "xyz@abc.com"
)

// Subscription Attribute Modifier
type subAttrModifier func(*map[string]string)

func subAttributes(m ...subAttrModifier) *map[string]string {
	attr := &map[string]string{}

	for _, f := range m {
		f(attr)
	}

	return attr
}

func withSubARN(s *string) subAttrModifier {
	return func(attr *map[string]string) {
		(*attr)["SubscriptionArn"] = *s
	}
}

func withSubOwner(s *string) subAttrModifier {
	return func(attr *map[string]string) {
		(*attr)["Owner"] = *s
	}
}

// subscription Observation Modifer
type subObservationModifier func(*v1alpha1.SNSSubscriptionObservation)

func subObservation(m ...func(*v1alpha1.SNSSubscriptionObservation)) *v1alpha1.SNSSubscriptionObservation {
	o := &v1alpha1.SNSSubscriptionObservation{}

	for _, f := range m {
		f(o)
	}
	return o
}

func withSubObservationArn(s *string) subObservationModifier {
	return func(o *v1alpha1.SNSSubscriptionObservation) {
		o.Arn = s
	}
}

func withSubObservationOwner(s *string) subObservationModifier {
	return func(o *v1alpha1.SNSSubscriptionObservation) {
		o.Owner = s
	}
}

// func TestGetSNSSubscription(t *testing.T) {
// 	type args struct {
// 		client SubscriptionClient
// 	}

// 	cases := map[string]struct {
// 		args
// 		want
// 	}{
// 		"ListSubscriptionsError": {
// 			args: args{
// 				client: &fake.MockSubnetClient{
// 					MockListSubscriptionsRequest: func(input *awssns.ListSubscriptionsInput) awssns.ListSubscriptionsRequest {
// 						return awssns.ListSubscriptionsRequest{
// 							HTTPRequest: &http.Request{},
// 							Error:       errBoom,
// 						}
// 					}
// 				},
// 			},
// 		},
// 	}
// 	for name, tc := range cases {
// 		t.Run(name, func(t *testing.T) {
// 			topic, err := TestGetSNSSubscription(context.Background(), tc.args.client, topicArn)
// 			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
// 				t.Errorf("r: -want, +got\n%s", diff)
// 			}
// 			if diff := cmp.Diff(tc.want.topic, topic); diff != "" {
// 				t.Errorf("r: -want, +got:\n%s", diff)
// 			}

// 		})
// 	}
// }

func TestGenerateSubscribeInput(t *testing.T) {
	cases := map[string]struct {
		in  v1alpha1.SNSSubscriptionParameters
		out awssns.SubscribeOutput
	}{
		"FilledInput": {
			in:  v1alpha1.SNSSubscriptionParameters{},
			out: awssns.SubscribeOutput{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			input := GenerateSubscribeInput(&tc.in)
			if diff := cmp.Diff(input, &tc.out); diff != "" {
				t.Errorf("GenerateSubscribeInput(...): -want, +got\n:%s", diff)
			}
		})
	}
}

// func TestLateInitializeSubscription(t *testing.T) {
// 	type args struct {
// 		spec *v1alpha1.SNSSubscriptionParameters
// 		in   sns.Subscription
// 		attr map[string]string
// 	}

// 	cases := map[string]struct {
// 		args args
// 		want *v1alpha1.SNSSubscriptionParameters
// 	}{
// 		"AllFilledNoDiff": {
// 			args: args{
// 				spec: &v1alpha1.SNSSubscriptionParameters{}
// 				in:   *subscription(),
// 			},
// 			want: subParams(),
// 		},
// 	}

// 	for name, tc := range cases {
// 		t.Run(name, func(t *testing.T) {
// 			TestLateInitializeSubscription(tc.args.spec, tc.args.in, tc.args.attr)
// 			if diff := cmp.Diff(tc.args.spec, tc.want); diff != "" {
// 				t.Errorf("LateInitializeTopic(...): -want, +got:\n%s", diff)
// 			}
// 		})
// 	}
// }

func TestGetChangedSubAttributes(t *testing.T) {

	type args struct {
		p    v1alpha1.SNSSubscriptionParameters
		attr *map[string]string
	}

	cases := map[string]struct {
		args args
		want *map[string]string
	}{
		"NoChange": {
			args: args{
				p: v1alpha1.SNSSubscriptionParameters{
					Protocol: &subEmailProtocol,
					Endpoint: &subEmailEndpoint,
				},
				attr: subAttributes(),
			},
			want: subAttributes(),
		},
		"Change": {
			args: args{
				p: v1alpha1.SNSSubscriptionParameters{
					Protocol: &subEmailProtocol,
					Endpoint: &subEmailEndpoint,
				},
				attr: subAttributes(),
			},
			want: subAttributes(),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := GetChangedSubAttributes(tc.args.p, *tc.args.attr)
			if diff := cmp.Diff(*tc.want, c); diff != "" {
				t.Errorf("GetChangedSubAttributes(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestGenerateSubscriptionObservation(t *testing.T) {
	cases := map[string]struct {
		in  *map[string]string
		out *v1alpha1.SNSSubscriptionObservation
	}{
		"AllFilled": {
			in: subAttributes(
				withSubARN(&subName),
				withSubOwner(&subOwner),
			),
			out: subObservation(
				withSubObservationArn(&subName),
				withSubObservationOwner(&subOwner),
			),
		},
		"NoSubscriptions": {
			in: subAttributes(
				withSubOwner(&subOwner),
				withSubARN(&subName),
			),
			out: subObservation(
				withSubObservationArn(&subName),
				withSubObservationOwner(&subOwner),
			),
		},
		// "Empty": {
		// 	in:  subAttributes(),
		// 	out: subObservation(),
		// },
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			observation := GenerateSubscriptionObservation(*tc.in)
			if diff := cmp.Diff(*tc.out, observation); diff != "" {
				t.Errorf("GenerateSubscriptionObservation(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestIsSNSSubscriptionUpToDate(t *testing.T) {
	type args struct {
		p    v1alpha1.SNSSubscriptionParameters
		attr *map[string]string
	}

	cases := map[string]struct {
		args args
		want bool
	}{
		"SameFieldsAndAllFilled": {
			args: args{
				attr: subAttributes(),
				p:    v1alpha1.SNSSubscriptionParameters{},
			},
			want: true,
		},
		// NOTE: Presently we don't have support for any attributes.
		// So there is no way to test this case. Putting it here for book keeping
		//
		// "DifferentFields": {
		// 	args: args{
		// 		attr: subAttributes(),
		// 		p:    v1alpha1.SNSSubscriptionParameters{},
		// 	},
		// 	want: false,
		// },
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsSNSSubscriptionUpToDate(tc.args.p, *tc.args.attr)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Topic : -want, +got:\n%s", diff)
			}
		})
	}
}

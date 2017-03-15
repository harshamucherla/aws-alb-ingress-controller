package controller

import (
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
)

// ELBV2 is our extension to AWS's elbv2.ELBV2
type ELBV2 struct {
	svc elbv2iface.ELBV2API
}

type Tags []*elbv2.Tag

func newELBV2(awsconfig *aws.Config) *ELBV2 {
	awsSession, err := session.NewSession(awsconfig)
	if err != nil {
		AWSErrorCount.With(prometheus.Labels{"service": "ELBV2", "request": "NewSession"}).Add(float64(1))
		glog.Errorf("Failed to create AWS session. Error: %s.", err.Error())
		return nil
	}

	elbClient := ELBV2{
		elbv2.New(awsSession),
	}
	return &elbClient
}

func (elb *ELBV2) describeLoadBalancers(clusterName *string) ([]*elbv2.LoadBalancer, error) {
	var loadbalancers []*elbv2.LoadBalancer
	describeLoadBalancersInput := &elbv2.DescribeLoadBalancersInput{
		PageSize: aws.Int64(100),
	}

	for {
		describeLoadBalancersOutput, err := elb.svc.DescribeLoadBalancers(describeLoadBalancersInput)
		if err != nil {
			return nil, err
		}

		describeLoadBalancersInput.Marker = describeLoadBalancersOutput.NextMarker

		for _, loadBalancer := range describeLoadBalancersOutput.LoadBalancers {
			if strings.HasPrefix(*loadBalancer.LoadBalancerName, *clusterName+"-") {
				if s := strings.Split(*loadBalancer.LoadBalancerName, "-"); len(s) == 2 {
					if s[0] == *clusterName {
						loadbalancers = append(loadbalancers, loadBalancer)
					}
				}
			}
		}

		if describeLoadBalancersOutput.NextMarker == nil {
			break
		}
	}
	return loadbalancers, nil
}

func (elb *ELBV2) describeTargetGroups(loadBalancerArn *string) ([]*elbv2.TargetGroup, error) {
	var targetGroups []*elbv2.TargetGroup
	describeTargetGroupsInput := &elbv2.DescribeTargetGroupsInput{
		LoadBalancerArn: loadBalancerArn,
		PageSize:        aws.Int64(100),
	}

	for {
		describeTargetGroupsOutput, err := elb.svc.DescribeTargetGroups(describeTargetGroupsInput)
		if err != nil {
			return nil, err
		}

		describeTargetGroupsInput.Marker = describeTargetGroupsOutput.NextMarker

		for _, targetGroup := range describeTargetGroupsOutput.TargetGroups {
			targetGroups = append(targetGroups, targetGroup)
		}

		if describeTargetGroupsOutput.NextMarker == nil {
			break
		}
	}
	return targetGroups, nil
}

func (elb *ELBV2) describeListeners(loadBalancerArn *string) ([]*elbv2.Listener, error) {
	var listeners []*elbv2.Listener
	describeListenersInput := &elbv2.DescribeListenersInput{
		LoadBalancerArn: loadBalancerArn,
		PageSize:        aws.Int64(100),
	}

	for {
		describeListenersOutput, err := elb.svc.DescribeListeners(describeListenersInput)
		if err != nil {
			AWSErrorCount.With(prometheus.Labels{"service": "ELBV2", "request": "DescribeListeners"}).Add(float64(1))
			return nil, err
		}

		describeListenersInput.Marker = describeListenersOutput.NextMarker

		for _, listener := range describeListenersOutput.Listeners {
			listeners = append(listeners, listener)
		}

		if describeListenersOutput.NextMarker == nil {
			break
		}
	}
	return listeners, nil
}

func (elb *ELBV2) describeTags(arn *string) (Tags, error) {
	describeTags, err := elb.svc.DescribeTags(&elbv2.DescribeTagsInput{
		ResourceArns: []*string{arn},
	})

	var tags []*elbv2.Tag
	for _, tag := range describeTags.TagDescriptions[0].Tags {
		tags = append(tags, &elbv2.Tag{Key: tag.Key, Value: tag.Value})
	}

	return tags, err
}

func (elb *ELBV2) describeTargetGroupTargets(arn *string) (AwsStringSlice, error) {
	var targets AwsStringSlice
	targetGroupHealth, err := elbv2svc.svc.DescribeTargetHealth(&elbv2.DescribeTargetHealthInput{
		TargetGroupArn: arn,
	})
	if err != nil {
		return nil, err
	}
	for _, targetHealthDescription := range targetGroupHealth.TargetHealthDescriptions {
		targets = append(targets, targetHealthDescription.Target.Id)
	}
	sort.Sort(targets)
	return targets, err
}

func (elb *ELBV2) describeRules(listenerArn *string) ([]*elbv2.Rule, error) {
	describeRulesInput := &elbv2.DescribeRulesInput{
		ListenerArn: listenerArn,
	}

	describeRulesOutput, err := elb.svc.DescribeRules(describeRulesInput)
	if err != nil {
		return nil, err
	}

	return describeRulesOutput.Rules, nil
}

func (t *Tags) Get(s string) (string, bool) {
	for _, tag := range *t {
		if *tag.Key == s {
			return *tag.Value, true
		}
	}
	return "", false
}
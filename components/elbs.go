/*
Functions dealing with AWS ELBs used by ECS services.

Womply, www.womply.com
*/
package components

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elb"
)

//
// Takes a list of ELB names, retrieves their details and prints.
//
func PrintElbs(creds *credentials.Credentials, region string, loadBalancers []*string) {
	if len(loadBalancers) > 0 {
		fmt.Println("")
		PrintSeparator()
		balancerInfo := GetElbData(creds, region, loadBalancers)
		for _, balancer := range balancerInfo.LoadBalancerDescriptions {
			fmt.Println("  Load Balancer:", *balancer.LoadBalancerName)
			fmt.Println("  - DNSName:", *balancer.DNSName)
			for _, instance := range balancer.Instances {
				fmt.Println("  - Instance:", *instance.InstanceId)
			}
			for _, backend := range balancer.BackendServerDescriptions {
				fmt.Println("  - Backend server port:", *backend.InstancePort)
				fmt.Println("  - Backend server:", backend.String())
			}
		}
	}
}

//
// Fetch ELB data
//
func GetElbData(creds *credentials.Credentials, region string, loadBalancers []*string) *elb.DescribeLoadBalancersOutput {
	// We need to get a new client because the "ELB" service is different from the "ECS" service.
	elbAwsConn := elb.New(session.New(), &aws.Config{
		Region:      aws.String(region),
		Credentials: creds,
	})

	// Fetch the details given the list of ELBs.
	balancerInfo, err := elbAwsConn.DescribeLoadBalancers(&elb.DescribeLoadBalancersInput{
		LoadBalancerNames: loadBalancers,
	})
	CheckError("fetching load balancer data", err)
	return balancerInfo
}

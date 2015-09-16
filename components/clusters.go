/*
Functions dealing with ECS clusters.

Womply, www.womply.com
*/
package components

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/ecs"
)

//
// List the user-visible clusters and their service names.
//
func ListClusters(creds *credentials.Credentials, region string) {
	awsConn := GetEcsConnection(creds, region)
	clusterList, err := awsConn.ListClusters(&ecs.ListClustersInput{})
	CheckError("fetching clusters list", err)
	clusters, err := awsConn.DescribeClusters(&ecs.DescribeClustersInput{Clusters: clusterList.ClusterArns})
	CheckError("fetching cluster data", err)
	for _, cluster := range clusters.Clusters {
		PrintSeparator()
		fmt.Printf("Cluster: %s (%s)\n", *cluster.ClusterName, *cluster.Status)
		fmt.Println(" ", *cluster.ActiveServicesCount, "services active,", *cluster.RegisteredContainerInstancesCount, "containers")
		fmt.Println("  Tasks:", *cluster.RunningTasksCount, "running,", *cluster.PendingTasksCount, "pending")
		serviceList, err := awsConn.ListServices(&ecs.ListServicesInput{Cluster: cluster.ClusterArn})
		CheckError(fmt.Sprintf("finding services for cluster %s", *cluster.ClusterName), err)
		serviceInfo, err := awsConn.DescribeServices(&ecs.DescribeServicesInput{
			Cluster:  cluster.ClusterArn,
			Services: serviceList.ServiceArns,
		})
		CheckError(fmt.Sprintf("fetching service data for cluster %s", *cluster.ClusterName), err)
		for _, service := range serviceInfo.Services {
			fmt.Printf("  - Service: %s (%s), running count: %d\n", *service.ServiceName, *service.Status, *service.RunningCount)
		}
	}
}

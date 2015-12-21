/*
Implements the functions that work with ECS services.

Womply, www.womply.com
*/
package components

import str "strings"
import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/ecs"
)

//
// Fetches and prints the information about the services. It also collects the names of the load balancers in use and
// returns them so that the printElbs() function can use them if desired.
//
func PrintServices(creds *credentials.Credentials,
	region string,
	clusterName string,
	serviceName string,
	verboseFlag bool,
	eventsFlag int) []*string {

	var foundService = false

	// Create a client connection object.
	awsConn := GetEcsConnection(creds, region)

	// Fetch the list of services in this cluster.
	resp, err := awsConn.ListServices(&ecs.ListServicesInput{Cluster: &clusterName})
	CheckError(fmt.Sprintf("finding services for cluster %s", clusterName), err)

	fmt.Println(len(resp.ServiceArns), "Services in cluster", clusterName)

	// Retrieve the details given the list of services.
	serviceInfo, err := awsConn.DescribeServices(&ecs.DescribeServicesInput{
		Cluster:  &clusterName,
		Services: resp.ServiceArns,
	})
	CheckError(fmt.Sprintf("fetching service data for cluster %s", clusterName), err)

	var loadBalancers = make([]*string, 0) // Collect the load balancers from each service

	if len(serviceInfo.Services) == 0 {
		fmt.Println("  No services to describe.")
	} else {
		// Loop through and print information for each service
		for _, service := range serviceInfo.Services {
			// If a service name was passed in, only print that service's info; skip the others
			if (serviceName == "") || (serviceName == *service.ServiceName) {
				PrintSeparator()
				fmt.Println("  Service:", *service.ServiceName)
				fmt.Println("  - Running Count:", *service.RunningCount)
				fmt.Println("  - Status:", *service.Status)
				for _, balancer := range service.LoadBalancers {
					loadBalancers = append(loadBalancers, balancer.LoadBalancerName) // Add to our list for returning
					fmt.Println("  - Load Balancer:", *balancer.LoadBalancerName, "Port:", *balancer.ContainerPort)
					fmt.Println("    Container Name:", *balancer.ContainerName)
				}
				for _, depl := range service.Deployments {
					fmt.Println("  - Deployment:", *depl.Id, "Status:", *depl.Status)
					fmt.Println("    Running instances:", *depl.RunningCount)
				}
				PrintServiceTasks(awsConn, clusterName, *service.ServiceName, *service.TaskDefinition)
				if (eventsFlag > 0) && (len(service.Events) > 0) {
					fmt.Printf("  - Events (most recent %d):\n", eventsFlag)
					if len(service.Events) < 10 {
						eventsFlag = len(service.Events)
					}
					for eventCount := 0; eventCount < eventsFlag; eventCount++ {
						fmt.Printf("    At %s: %s\n", *service.Events[eventCount].CreatedAt, *service.Events[eventCount].Message)
						//  Check if the message is about a task. If so, let's get the task ID and print info about the task
						var eventMessage = *service.Events[eventCount].Message
						var pos = str.Index(eventMessage, "(task ")
						// If the message is about a task, let's print the basic info about the task
						if pos != -1 {
							var taskID = eventMessage[pos+6 : len(eventMessage)-2]
							eventInfo, err := awsConn.DescribeTasks(&ecs.DescribeTasksInput{
								Tasks:   []*string{&taskID},
								Cluster: &clusterName,
							})
							CheckError(fmt.Sprintf("getting task data for %s\n", taskID), err)
							if len(eventInfo.Tasks) < 1 {
								fmt.Println("      Request for task data returned no results.")
							} else {
								fmt.Println("      Task:", *eventInfo.Tasks[0].TaskDefinitionArn)
								fmt.Println("      Last known status:", *eventInfo.Tasks[0].LastStatus)
							}
						}
					}
				}
				PrintTaskDefinition(awsConn, service.TaskDefinition, verboseFlag)
			}
			if serviceName == *service.ServiceName {
				foundService = true
			}
		}
	}
	// Hmm, we didn't come across the service that was asked for.
	if (serviceName != "") && (foundService == false) {
		fmt.Println("  Service", serviceName, "not found.")
	}
	return loadBalancers
}

//
// Update a service by specifying a new image URL, which will register a new task definition revision and update the service,
// meaning the service instances are restarted. If the image URL starts with a colon (:) then it will get the current image URL,
// and update the service with its current URL using the string as the tag. For example, passing ":latest" will update the
// service with the same image, tagged 'latest'.
//
func UpdateService(creds *credentials.Credentials, region string, clusterName string, serviceName string, newImage string) {
	if newImage == "" {
		fmt.Println("Error: You must specify a new image URL to update the image!")
		os.Exit(1)
	}
	var updateTag = str.HasPrefix(newImage, ":")

	fmt.Println("Updating service", serviceName)
	awsConn := GetEcsConnection(creds, region)
	// Get the service, extract task definition
	serviceInfo, err := awsConn.DescribeServices(&ecs.DescribeServicesInput{
		Cluster:  &clusterName,
		Services: []*string{&serviceName},
	})
	CheckError(fmt.Sprintf("fetching service data for service %s", serviceName), err)
	if len(serviceInfo.Services) == 0 {
		fmt.Printf("Error: Got zero services for name %s\n", serviceName)
		os.Exit(1)
	}
	// Get the task definition description
	taskDefn, err := awsConn.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{TaskDefinition: serviceInfo.Services[0].TaskDefinition})
	// Right now we only support tasks with a single container.
	if len(taskDefn.TaskDefinition.ContainerDefinitions) > 1 {
		fmt.Println("Apologies, this service has multiple containers and I only support one right now.")
		os.Exit(1)
	}
	// Update the image URL
	if updateTag {
		var urlParts = str.Split(*taskDefn.TaskDefinition.ContainerDefinitions[0].Image, ":")
		// If we get more than two parts, we can't safely append the image tag so let's bail out.
		if len(urlParts) > 2 {
			fmt.Println("Split on colon found more than two elements in current image URL")
			os.Exit(1)
		}
		newImage = fmt.Sprintf("%s%s", urlParts[0], newImage) // Since newImage starts with a colon, we can just append
	}

	fmt.Println("  - Task Definition:", *taskDefn.TaskDefinition.Family)
	fmt.Println("  - Current image:", *taskDefn.TaskDefinition.ContainerDefinitions[0].Image)
	fmt.Println("  - Updating to:", newImage)
	*taskDefn.TaskDefinition.ContainerDefinitions[0].Image = newImage

	// Register the task definition
	taskDefinitionOutput, err := awsConn.RegisterTaskDefinition(&ecs.RegisterTaskDefinitionInput{
		ContainerDefinitions: taskDefn.TaskDefinition.ContainerDefinitions,
		Family:               taskDefn.TaskDefinition.Family,
		Volumes:              taskDefn.TaskDefinition.Volumes,
	})
	CheckError("registering updated task definition", err)
	fmt.Println("  -> Task definition updated, registered as revision", *taskDefinitionOutput.TaskDefinition.Revision)

	// Update the service
	updateServiceOutput, err := awsConn.UpdateService(&ecs.UpdateServiceInput{
		Cluster:        &clusterName,
		Service:        &serviceName,
		TaskDefinition: taskDefinitionOutput.TaskDefinition.TaskDefinitionArn,
	})
	CheckError("updating service with new task definition", err)
	fmt.Println("  -> Service updated with new task definition:")
	fmt.Println("     - Desired count:", *updateServiceOutput.Service.DesiredCount)
	fmt.Println("     - Pending count:", *updateServiceOutput.Service.PendingCount)
	fmt.Println("     - Running count:", *updateServiceOutput.Service.RunningCount)
	fmt.Println("     - Service status:", *updateServiceOutput.Service.Status)
}

//
// Check the status of a service by fetching the tasks and comparing task definitions and run state
// to see if tasks are running the same task definition revision that the service is associated with.
//
func CheckService(creds *credentials.Credentials, region string, clusterName string, serviceName string, verboseFlag bool) {
	// get the service task definition
	awsConn := GetEcsConnection(creds, region)
	// Get the service, extract task definition
	serviceInfo, err := awsConn.DescribeServices(&ecs.DescribeServicesInput{
		Cluster:  &clusterName,
		Services: []*string{&serviceName},
	})
	CheckError("getting service information", err)
	if len(serviceInfo.Services) < 1 {
		fmt.Println("Attempt to fetch service data for", serviceName, "returned no data!")
		os.Exit(1)
	}
	serviceDef := serviceInfo.Services[0]

	taskWarnings, taskRunning := CheckServiceTasks(awsConn, clusterName, serviceName, verboseFlag, *serviceDef.TaskDefinition)
	for _, msg := range taskWarnings {
		fmt.Println(msg)
	}

	var elbCount = 0
	var balancerNames = make([]*string, 0)
	for _, bals := range serviceDef.LoadBalancers {
		balancerNames = append(balancerNames, bals.LoadBalancerName)
	}
	serviceElbs := GetElbData(creds, region, balancerNames)
	for _, balancer := range serviceElbs.LoadBalancerDescriptions {
		elbCount += len(balancer.Instances)
	}
	if elbCount != taskRunning {
		fmt.Println("WARNING: ELB instance count of", elbCount, "is different from number of running tasks", taskRunning)
	}
}

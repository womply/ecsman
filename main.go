/*
Package main implements the ECSMAN command-line utility for managing
Amazon Web Services' EC2 Container Service.

Womply, www.womply.com
*/
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/masonoise/ecsman/components"

	"github.com/aws/aws-sdk-go/aws/credentials"
)

/*
Main entry point. Check command line arguments using the Flag module, determine
credentials to be used, set things up, then call the appropriate function handler
depending on what the user's asking for.

Usage:
	ecsman ls == list clusters in the region
	ecsman <options> ls clusterName == list services in the cluster
	ecsman <options> ls clusterName serviceName == list service details
	ecsman <options> check clusterName serviceName == check service tasks
	ecsman <options> update clusterName serviceName imageURL == update the service with new image
	ecsman <options> taskdefs == list task definitions
	ecsman <options> register taskFile == register a task using the specified JSON file
	ecsman <options> run clusterName taskName == run a task
*/
func main() {
	const VERSION string = "1.0.2"

	verboseFlag := flag.Bool("v", false, "Verbose printing with details")
	versionFlag := flag.Bool("version", false, "Display version and exit")
	regionFlag := flag.String("region", "us-west-2", "AWS region")
	elbFlag := flag.Bool("elb", false, "Print ELB information")
	credFlag := flag.String("cred", "", "AWS credential profile name (or use ECSCREDENTIAL env var)")
	eventsFlag := flag.Int("events", 0, "List events for a service")
	flag.Usage = usage
	flag.Parse()

	if *versionFlag == true {
		fmt.Println("Version", VERSION)
		os.Exit(0)
	}

	// Get the operation requested
	if flag.NArg() < 1 {
		usage()
		os.Exit(0)
	}
	var operation = flag.Arg(0)

	// What it calls "shared credentials" is the object that handles reading a user's credentials file from ~/.aws/credentials
	// First, figure out whether we use a profile name passed in as a command-line argument, an environment variable,
	// or the "default" profile. If the cred flag passed in is "env" then it will expect to find the environment
	// vars AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY to use. Otherwise the flag is expected to be the profile name.
	var credProfile = "default"
	if *credFlag == "" {
		if os.Getenv("ECSCREDENTIAL") != "" {
			credProfile = os.Getenv("ECSCREDENTIAL")
		}
	} else {
		if *credFlag != "env" {
			credProfile = *credFlag
		}
	}
	if *verboseFlag {
		if *credFlag == "env" {
			fmt.Printf("--> Running with credentials from environment variables\n\n")
		} else {
			fmt.Printf("--> Running with credential profile %s\n\n", credProfile)
		}
	}
	var creds = credentials.NewSharedCredentials("", credProfile)

	// Okay, what do we want to do today?
	switch {
	case operation == "ls" && flag.NArg() < 2: // ls without a cluster name
		components.ListClusters(creds, *regionFlag)
	case operation == "ls" && flag.NArg() > 1: // ls with cluster name and maybe service name
		var serviceName = ""
		if flag.NArg() > 2 {
			serviceName = flag.Arg(2)
		}
		var loadBalancers []*string // Save from printServices in case user wants the ELB info printed too.
		loadBalancers = components.PrintServices(creds, *regionFlag, flag.Arg(1), serviceName, *verboseFlag, *eventsFlag)
		if *elbFlag {
			components.PrintElbs(creds, *regionFlag, loadBalancers)
		}
	case operation == "register":
		if flag.NArg() < 2 { // Make sure there's a task.JSON filename provided
			usageMsg("Must specify JSON file describing the task to register.")
		}
		components.CreateTask(creds, *regionFlag, flag.Arg(1))
	case operation == "update":
		if flag.NArg() < 4 { // Need cluster name, service name, and image URL
			usageMsg("Must specify cluster name, service name, and image URL to update.")
		}
		components.UpdateService(creds, *regionFlag, flag.Arg(1), flag.Arg(2), flag.Arg(3))
	case operation == "check":
		if flag.NArg() < 3 { // Need cluster name and service name
			usageMsg("Must specify cluster name and service name to check.")
		}
		components.CheckService(creds, *regionFlag, flag.Arg(1), flag.Arg(2), *verboseFlag)
	case operation == "run":
		if flag.NArg() < 3 { // Make sure there's a cluster name and  task name provided
			usageMsg("Must specify a cluster name and the task name to run.")
		}
		components.RunTask(creds, *regionFlag, flag.Arg(1), flag.Arg(2))
	case operation == "taskdefs":
		if flag.NArg() < 2 {
			components.PrintTasks(creds, *regionFlag, "", "")
		} else {
			if flag.NArg() < 3 {
				components.PrintTasks(creds, *regionFlag, flag.Arg(1), "")
			} else {
				components.PrintTasks(creds, *regionFlag, flag.Arg(1), flag.Arg(2))
			}
		}
	case operation != "":
		usageMsg(fmt.Sprintf("Unknown operation: %s", operation))
	}

}

func usage() {
	fmt.Println("Usage: ecsman <flags> <operation> <cluster> <service>")
	fmt.Println("\n  Operations: ls, update, check, register, run, taskdefs")
	fmt.Println("    ls: list. Cluster, service are optional to limit the listing.")
	fmt.Println("    update: update a service. Requires cluster, service, image URL.")
	fmt.Println("    check: check a service healt. Requires cluster, service.")
	fmt.Println("    register: register a task definition. Requires task def JSON file path.")
	fmt.Println("    run: run a task. Requires cluster and task name.")
	fmt.Println("    taskdefs: list task definitions. Task family name and revision are optional. See documentation.")
	fmt.Println("\n  Flags:")
	fmt.Println("    -v                 For verbose listings with more details.")
	fmt.Println("    -elb               List ELB information with cluster. Defaults to false.")
	fmt.Println("    -events <int>      List <int> events for a service. Defaults to 0.")
	fmt.Println("    -cred <profile>    AWS credential profile name (or use ECSCREDENTIAL env var)")
	fmt.Println("    -region <region>   AWS region, defaults to us-west-2")
	fmt.Println("    -version           Print program version and exit.")
}

func usageMsg(msg string) {
	fmt.Println(msg)
	usage()
	os.Exit(1)
}

/*
Implements the functions that work with ECS tasks and task definitions.

Womply, www.womply.com
*/
package components

import str "strings"
import (
	"fmt"
	"io/ioutil"
	"os"

	"encoding/json"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/ecs"
)

//
// Function to print the details of a service's task definition, since it's got a lot of fiddly details.
//
func PrintTaskDefinition(awsConn *ecs.ECS, taskDefinition *string, verboseFlag bool) {
	// Fetch the details of the task definition.
	taskDef, err := awsConn.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
		TaskDefinition: taskDefinition,
	})
	CheckError(fmt.Sprintf("fetching Task Definition for %s", *taskDefinition), err)
	fmt.Println("  - Task Definition:", *taskDefinition)
	fmt.Println("    - Family:", *taskDef.TaskDefinition.Family)
	for _, containerDef := range taskDef.TaskDefinition.ContainerDefinitions {
		fmt.Println("    - Container Definition:")
		fmt.Println("      - Image:", *containerDef.Image)
		if verboseFlag {
			fmt.Println("      - CPU:", *containerDef.Cpu)
			fmt.Println("      - Memory:", *containerDef.Memory)
		}
		for _, portMap := range containerDef.PortMappings {
			fmt.Println("      - Container Port", *portMap.ContainerPort, ": Host Port", *portMap.HostPort)
		}
		if len(containerDef.Command) > 0 {
			fmt.Printf("      - Command: %v\n", containerDef.Command)
		}
		if len(containerDef.EntryPoint) > 0 {
			fmt.Printf("      - Entry Point: %v\n", containerDef.EntryPoint)
		}
		if (len(containerDef.Environment) > 0) && (verboseFlag) {
			fmt.Println("      - Environment:")
			for _, envVariable := range containerDef.Environment {
				fmt.Println("       ", *envVariable.Name, "=", *envVariable.Value)
			}
		}
	}
}

//
// Print information about the tasks associated with the specific service.
//
func PrintServiceTasks(awsConn *ecs.ECS, clusterName string, serviceName string, taskDefinition string) {
	taskRevision := getRevisionFromTaskDefinition(taskDefinition)
	tasks := getServiceTasks(awsConn, clusterName, serviceName)
	for _, task := range tasks {
		fmt.Println("  - Task", *task.TaskArn)
		fmt.Println("    Task Def:", *task.TaskDefinitionArn)
		fmt.Println("    Desired status", *task.DesiredStatus, "- Last status", *task.LastStatus)
		if taskRevision != getRevisionFromTaskDefinition(*task.TaskDefinitionArn) {
			fmt.Println("    *** WARNING: task does not have the same task/revision as the service definition ***")
		}
	}
}

func CheckServiceTasks(awsConn *ecs.ECS, clusterName string,
	serviceName string, verboseFlag bool, serviceTaskDefinition string) ([]string, int) {
	taskWarnings := make([]string, 0)

	serviceTaskRevision := getRevisionFromTaskDefinition(serviceTaskDefinition)
	tasks := getServiceTasks(awsConn, clusterName, serviceName)

	var taskRunning = 0
	for _, task := range tasks {
		if verboseFlag {
			taskWarnings = append(taskWarnings, fmt.Sprintln("  - Task", *task.TaskArn))
			taskWarnings = append(taskWarnings, fmt.Sprintln("    Task Def:", *task.TaskDefinitionArn))
			taskWarnings = append(taskWarnings, fmt.Sprintln("    Desired status", *task.DesiredStatus, "- Last status", *task.LastStatus))
		}
		if *task.LastStatus == "RUNNING" {
			taskRunning += 1
		}
		taskRevision := getRevisionFromTaskDefinition(*task.TaskDefinitionArn)
		if serviceTaskRevision != taskRevision {
			taskWarnings = append(taskWarnings, fmt.Sprintln("WARNING: task uses", taskRevision, "but service definition is", serviceTaskRevision))
		}
	}
	if taskRunning == 0 {
		taskWarnings = append(taskWarnings, fmt.Sprintln("WARNING: No tasks in RUNNING state for the service"))
	}
	return taskWarnings, taskRunning
}

//
// Run a task - runs 1 instance of the specified task.
//
func RunTask(creds *credentials.Credentials, region string, clusterName string, taskName string) {
	fmt.Println("Running one instance of task", taskName)
	awsConn := GetEcsConnection(creds, region)
	var startedBy = "ecsman"
	var runCount = int64(1)
	runTaskOutput, err := awsConn.RunTask(&ecs.RunTaskInput{
		Cluster:        &clusterName,
		Count:          &runCount,
		StartedBy:      &startedBy,
		TaskDefinition: &taskName,
	})
	CheckError("running task", err)
	for _, fail := range runTaskOutput.Failures {
		fmt.Println("  FAILED Task:", *fail.Arn)
		fmt.Println("  - Error:", *fail.Reason)
	}
	for _, task := range runTaskOutput.Tasks {
		var containerNames = make([]string, 0)
		for i := 0; i < len(task.Containers); i++ {
			containerNames = append(containerNames, *task.Containers[i].Name)
		}
		fmt.Println("  Running task definition:", *task.TaskDefinitionArn)
		fmt.Printf("  - Task Running on container(s) %v\n", containerNames)
		fmt.Println("  - Last known status:", *task.LastStatus)
	}
}

//
// Print the task definitions
//
func PrintTasks(creds *credentials.Credentials, region string, taskFamily string, taskRevision string) {
	var wantedRevision string
	if taskRevision == "latest" {
		wantedRevision = "latest"
	} else {
		wantedRevision = fmt.Sprintf("%s:%s", taskFamily, taskRevision)
	}
	awsConn := GetEcsConnection(creds, region)
	listInput := ecs.ListTaskDefinitionsInput{}
	if taskFamily != "" {
		listInput.FamilyPrefix = &taskFamily
	}
	taskDefList, err := awsConn.ListTaskDefinitions(&listInput)
	CheckError("fetching task definitions list", err)

	families := map[string]string{}
	for taskIndex, taskdef := range taskDefList.TaskDefinitionArns {
		revision := getRevisionFromTaskDefinition(*taskdef)
		if taskFamily == "" { // Print a list of the families
			splitRevision := str.Split(revision, ":")
			families[splitRevision[0]] = splitRevision[1]
		} else {
			if wantedRevision == "latest" {
				if taskIndex+1 == len(taskDefList.TaskDefinitionArns) {
					PrintTaskDefinition(awsConn, taskdef, true)
				}
			} else {
				if (taskRevision == "") || (wantedRevision == revision) {
					PrintTaskDefinition(awsConn, taskdef, true)
				}
			}
		}
	}
	if taskFamily == "" {
		fmt.Println("Task Definition families:")
		for family, rev := range families {
			fmt.Printf("  %s (latest revision: %s)\n", family, rev)
		}
	}
}

//
// TaskDefn is a struct defining the layout of the Task Definition JSON file.
// This is used in order to parse the JSON file directly into a usable structure.
//
type TaskDefn struct {
	ContainerDefinitions []struct {
		Command      []string
		CPU          int64
		EntryPoint   []string
		Environment  []ecs.KeyValuePair
		Essential    bool
		Image        string
		Memory       int64
		Name         string
		PortMappings []ecs.PortMapping
	}
	Family string
}

//
// Read a task definition from JSON file and register it.
//
func CreateTask(creds *credentials.Credentials, region string, taskFile string) {
	fmt.Printf("Registering Task Definition...\n\n")
	// Create a client connection object.
	awsConn := GetEcsConnection(creds, region)

	// Pass the file in and get back the parsed container definition and the task family string
	containerDefinition, taskFamily := makeTaskDefinition(taskFile)

	containerDefs := []*ecs.ContainerDefinition{&containerDefinition}
	taskDefinitionOutput, err := awsConn.RegisterTaskDefinition(&ecs.RegisterTaskDefinitionInput{
		ContainerDefinitions: containerDefs,
		Family:               &taskFamily,
	})
	CheckError("registering task definition", err)

	fmt.Println("Registered new Task Definition:")
	fmt.Println("  - Family:", *taskDefinitionOutput.TaskDefinition.Family)
	fmt.Println("  - Revision:", *taskDefinitionOutput.TaskDefinition.Revision)
	fmt.Println("  - Status:", *taskDefinitionOutput.TaskDefinition.Status)
}

//
// Called by createTask() above. Given a filename, parse the JSON file and create a ContainerDefinition
// struct that can be passed to the SDK to create the task definition.
//
func makeTaskDefinition(taskFile string) (ecs.ContainerDefinition, string) {
	var taskJSON TaskDefn
	// Do it the easy way and read in the whole file. A JSON file's not going to be very large.
	fileBytes, err := ioutil.ReadFile(taskFile)
	CheckError(fmt.Sprintf("Error reading task file %s", taskFile), err)
	err = json.Unmarshal(fileBytes, &taskJSON)
	CheckError(fmt.Sprintf("Error reading task file %s", taskFile), err)
	// TODO: Support more than one if it's ever needed.
	if len(taskJSON.ContainerDefinitions) > 1 {
		fmt.Println("Right now I only support a single ContainerDefinition, sorry. Please edit and try again.")
		os.Exit(1)
	}

	// Now let's construct our Container Definition object, which is a lot of boring copying of fields.
	var defn ecs.ContainerDefinition
	defn.Cpu = &taskJSON.ContainerDefinitions[0].CPU
	var commands = make([]*string, 0)
	for i := 0; i < len(taskJSON.ContainerDefinitions[0].Command); i++ {
		commands = append(commands, &taskJSON.ContainerDefinitions[0].Command[i])
	}
	defn.Command = commands
	var entrypoints = make([]*string, 0)
	for i := 0; i < len(taskJSON.ContainerDefinitions[0].EntryPoint); i++ {
		entrypoints = append(entrypoints, &taskJSON.ContainerDefinitions[0].EntryPoint[i])
	}
	defn.EntryPoint = entrypoints
	var environments = make([]*ecs.KeyValuePair, 0)
	for i := 0; i < len(taskJSON.ContainerDefinitions[0].Environment); i++ {
		var keyval ecs.KeyValuePair
		keyval.Name = taskJSON.ContainerDefinitions[0].Environment[i].Name
		keyval.Value = taskJSON.ContainerDefinitions[0].Environment[i].Value
		environments = append(environments, &keyval)
	}
	defn.Environment = environments
	defn.Essential = &taskJSON.ContainerDefinitions[0].Essential
	defn.Image = &taskJSON.ContainerDefinitions[0].Image
	defn.Memory = &taskJSON.ContainerDefinitions[0].Memory
	defn.Name = &taskJSON.ContainerDefinitions[0].Name
	var ports = make([]*ecs.PortMapping, 0)
	for i := 0; i < len(taskJSON.ContainerDefinitions[0].PortMappings); i++ {
		var portmap ecs.PortMapping
		portmap.ContainerPort = taskJSON.ContainerDefinitions[0].PortMappings[i].ContainerPort
		portmap.HostPort = taskJSON.ContainerDefinitions[0].PortMappings[i].HostPort
		ports = append(ports, &portmap)
	}
	defn.PortMappings = ports
	return defn, taskJSON.Family
}

/////////////// Private functions

//
// Given a service, fetches the tasks associated with it and returns them in an array.
//
func getServiceTasks(awsConn *ecs.ECS, clusterName string, serviceName string) []*ecs.Task {
	taskOutput, err := awsConn.ListTasks(&ecs.ListTasksInput{
		Cluster:     &clusterName,
		ServiceName: &serviceName,
	})
	CheckError("fetching task list for service", err)
	var serviceTasks []*ecs.Task
	if len(taskOutput.TaskArns) > 0 {
		taskInfo, err := awsConn.DescribeTasks(&ecs.DescribeTasksInput{
			Tasks:   taskOutput.TaskArns,
			Cluster: &clusterName,
		})
		CheckError("fetching task data for service", err)
		serviceTasks = taskInfo.Tasks
	}
	return serviceTasks
}

// Given a task definition description, such as
// "arn:aws:ecs:us-west-2:751992077663:task-definition/demonstration:8"
// Return the name and revision, e.g. "demonstration:8"
func getRevisionFromTaskDefinition(taskDefinition string) string {
	splits := str.Split(taskDefinition, "/")
	if len(splits) > 1 {
		return splits[len(splits)-1]
	} else {
		return "unknown"
	}
}

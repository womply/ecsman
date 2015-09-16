
> ##**ECSMan**
---
A command-line ECS Manager (ecsman) utility to easily view and manage running services in Amazon Web Services EC2 Container Service clusters.

### Purpose

We're running microservices at Womply using ECS, and have found it to be a fairly easy infrastructure with which to operate. However, we did quickly find that it's difficult for developers to see what's going on, and deploying a simple update to a service is a big gap in usability -- it's not straightforward. Sure, we could set up IAM users for all of the developers on each team, give them console access, and let them work with things there, but that has many disadvantages, both in terms of user provisioning and access management and because to be honest the ECS interface is non-intuitive.

Hence the `ecsman` tool, which makes it easy to see what's running in a cluster, and describe the status of the services, ELBs, and tasks. Most importantly, though, it makes it very easy to deploy an update to a service with a single command that can also be hooked into our continuous integration servers.

### Installing

In the `bin` directory you'll find binaries for both OS X and Linux. Download the one you need and you should be good to go. If you prefer you can clone this repository and build your own, for example if you want to run this on Windows. Just install the dependencies, do a `go build -o bin/ecsman main.go` and you're set.

### AWS Credentials

Not surprisingly, using this utility requires having appropriate AWS credentials for the account / resources that you want to work with. The general best practice is to create an IAM user account with a policy providing the least privileges needed, which will be either read-only for a given ECS cluster, or read/write if you need the ability to create and update ECS services.

The utility expects you to have a `~/.aws/credentials` file with a profile containing the necessary keys. It will first check if you have specified a profile using the `-cred` command-line argument. If not, it will use the environment variable `ECSCREDENTIAL` if it is set. If that is blank, then it will fall back to trying a profile called `default`.

If you specify the special value `-cred env` the utility will expect to find the AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables set with the appropriate values. In this case no credentials file is needed. This can be useful when running the utility from a script, for example to update a service from a continuous deployment pipeline.

### Using

The utility is pretty self-explanatory. For most operations, run it with:

    ecsman <flags> <operation> <clusterName> <serviceName>

Where the flags and service name are optional. The cluster name is usually required. Running it with the `-h` flag will display usage details.

#### Flags:

* -cred <profile>

	The name of the credentials profile to use for AWS access. If you have a profile called "readonly" for example, you could specify `-cred readonly` on the command line. See the section above about credentials.

* -elb

	Include the ELB details when printing each service in the cluster. Defaults to false.

* -events <number>

	Include the most recent <number> events associated with each service when printing the details. Defaults to not printing any events.

* -v

	Show the verbose details. Without this, each service will show only basic task definition data. With this, it will show details such as environment variables and CPU/Memory settings. Defaults to false.

* -version

	Display the version of this utility, and exit.

* -h

	Print help information about the usage of the program and its flags.

#### Operations:

* ls \<cluster> \<service>

	List information about a region, cluster, or service. If `ls` is specified with no arguments, it will list the clusters visible in the region given the user credentials. If a cluster name is specified, it will list all of the services in the cluster. If a service name is specified, it will list the details for just that service.
	
	If the `-v` option is included, a verbose listing will be provided with more detail. If the `-events` option is included, the specified number of events will be displayed for each service. If the `-elb` option is included, the ELBs associated with the cluster will be displayed.

* update cluster service imageURL

	Update the specified service with the new image, where imageURL is the URL of the new Docker image for the service. The update is done by creating a new revision of the service's task definition based on the current one, changing only the image URL. Then the service is updated to use the new revision. This results in new instances of the service being spun up and the existing instances being shut down in a "rolling restart" manner.
	
	The image URL can be a full URL such as `hub.docker.com/acme/anvil` with an optional tag, or it can be a tag on its own, such as `:latest`. Note that a tag **must** begin with a colon (':'). In that case, the existing image URL will be changed to include the provided tag, replacing a previous tag, if any. This makes updates from one tag to another very easy. For example: `ecsman update mycluster myservice :newbuild` will take the existing image URL, add or change the tag to ":newbuild", and update the service.

* check cluster service

	This will fetch information about the specified service and its tasks, and do some basic checking of the service status. It will print a warning if there are no running tasks for the service, and it will also print a warning if any task is not running the same task definition and revision that the service is associated with. For example, if you try to update a service to a new task definition revision but lack the resources, you may see that the service specifies revision 8 while the running tasks are still showing revision 7.

* taskdefs \<family> \<revision>

	List the task definitions. If no family is specified, a list of the families and the latest revision for each will be shown. If a family is specified, only task definitions in that family will be shown. If a revision is specified, only that revision will be shown. Use "latest" to see only the latest revision.

* register filename

	Register a new task definition using a JSON file describing the task. This will read the provided file and register the task definition. If this is an existing task family, ECS will create a new revision. The family, revision, and status of the task definition will be printed when finished. See the provided "sample_task.json" file as an example. Note that this does not yet support every task definition field supported by ECS, but it supports the most important ones.

* run cluster taskname

	Used to run a task. It will run a single instance of the latest revision of the named task, and report the results.


### Examples

`ecsman ls prod`

Will show the services currently running in the ECS cluster called "prod". Each service will be printed with its name, number of running instances, load balancer name, deployments, and task definitions with container image and ports. Since the `-elb` flag defaults to false, this will not display the ELB details.

`ecsman ls prod my_api`

Will show only the service named "my_api" in the cluster "prod". Other services will not be shown. However, note that the header line will indicate that the "prod" cluster has X services, so you can determine if there are other services not being displayed.

`ecsman -elb ls prod`

Will show both the service and ELB information for the cluster "prod".

`ecsman -v ls prod`

Will show the service information for the cluster "prod" in verbose mode, with details of the service task definition.

`ecsman -events 5 ls prod my_api`

Will show the service details for service "my_api" in cluster "prod", and will include the most recent 5 ECS events for the service.

`ecsman register taskdef.json`

Will read the file "taskdef.json" and register the task definition accordingly.

`ecsman update prod my_api docker.io/myco/docker-image`

Will update the task definition for service "my_api" with the new Docker image URL, register a new revision of the task definition, and update the service.

`ecsman update prod my_api :latest`

Will register a new task definition for the service "my_api" using the "latest" tag for the existing image, and update the service.

`ecsman -cred env update prod my_api :latest`

Will update the service with the new image using AWS credentials found in environment variables (see Credentials section above). This is an example of how it could be run from an automated deployment script.

`ecsman check prod my_api`

Will display a warning if service "my_api" has no running tasks or if any task is running an incorrect task revision.

`ecsman taskdefs`

Will display a list of all Task Definition families. Includes the latest revision for each, to make it easy to see the status.

`ecsman taskdefs my_task 5`

Will display the Task Definition details for revision 5 of the "my_task" family.

`ecsman taskdefs my_task latest`

Will display the details of the latest revision of the Task Definition for "my_task".


### Notes

This is still a very beta-version utility, so there may be edge cases not caught, and error handling might not be as robust as it could be. Tread carefully and please submit issues or (even better) fixes.



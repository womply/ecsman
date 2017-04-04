/*
Utility functions.

Womply, www.womply.com
*/
package components

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
)

//
// Utility function to check if an error is non-nil. Prints a helpful message and exits if so.
// The helpful message is passed by the caller along with the error to check.
//
func CheckError(action string, err error) {
	if err != nil {
		fmt.Println("Error", action, "==>", err)
		os.Exit(1)
	}
}

// Little utility function since we print separators in a few places.
func PrintSeparator() {
	fmt.Println("-----------------------------------------------------------------------------------------------")
}

// Create a connection object for ECS service work.
func GetEcsConnection(creds *credentials.Credentials, region string) *ecs.ECS {
	return ecs.New(session.New(), &aws.Config{
		Region:      aws.String(region),
		Credentials: creds,
	})
}

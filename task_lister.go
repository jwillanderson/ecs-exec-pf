package ecsexecpf

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

type ecsTaskLister interface {
	ListTasks(context.Context, *ecs.ListTasksInput, ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
}

func ListTasksForService(cfg aws.Config, cluster, serviceName string) ([]string, error) {
	return listTasksWithLister(cluster, serviceName, ecs.NewFromConfig(cfg))
}

func listTasksWithLister(cluster, serviceName string, lister ecsTaskLister) ([]string, error) {
	var taskIds []string
	var nextToken *string

	for {
		input := &ecs.ListTasksInput{
			Cluster:     aws.String(cluster),
			ServiceName: aws.String(serviceName),
			NextToken:   nextToken,
		}

		output, err := lister.ListTasks(context.Background(), input)
		if err != nil {
			return nil, fmt.Errorf("failed to list tasks for service %s/%s: %w", cluster, serviceName, err)
		}

		for _, arn := range output.TaskArns {
			taskIds = append(taskIds, shortNameFromArn(arn))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	if len(taskIds) == 0 {
		return nil, fmt.Errorf("no running tasks found for service %s/%s", cluster, serviceName)
	}

	return taskIds, nil
}

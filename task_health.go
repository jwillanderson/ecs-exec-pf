package ecsexecpf

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type ecsTaskHealthChecker interface {
	DescribeTasks(context.Context, *ecs.DescribeTasksInput, ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
	ListTasks(context.Context, *ecs.ListTasksInput, ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
}

func PickHealthyTask(cfg aws.Config, cluster, service string) (string, error) {
	client := ecs.NewFromConfig(cfg)
	return pickHealthyTaskWithClient(context.Background(), cluster, service, client)
}

func pickHealthyTaskWithClient(ctx context.Context, cluster, service string, client ecsTaskHealthChecker) (string, error) {
	taskArns, err := listAllTaskArns(ctx, cluster, service, client)
	if err != nil {
		return "", err
	}

	if len(taskArns) == 0 {
		return "", fmt.Errorf("no running tasks found for service %s/%s", cluster, service)
	}

	input := &ecs.DescribeTasksInput{
		Cluster: aws.String(cluster),
		Tasks:   taskArns,
	}

	output, err := client.DescribeTasks(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to describe tasks: %w", err)
	}

	for _, task := range output.Tasks {
		if isTaskHealthy(task) {
			return shortNameFromArn(*task.TaskArn), nil
		}
	}

	return "", fmt.Errorf("no healthy tasks found for service %s/%s", cluster, service)
}

func listAllTaskArns(ctx context.Context, cluster, service string, client ecsTaskHealthChecker) ([]string, error) {
	var arns []string
	var nextToken *string

	for {
		output, err := client.ListTasks(ctx, &ecs.ListTasksInput{
			Cluster:       aws.String(cluster),
			ServiceName:   aws.String(service),
			DesiredStatus: types.DesiredStatusRunning,
			NextToken:     nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list tasks: %w", err)
		}
		arns = append(arns, output.TaskArns...)
		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}
	return arns, nil
}

func isTaskHealthy(task types.Task) bool {
	if task.LastStatus == nil || *task.LastStatus != "RUNNING" {
		return false
	}
	if task.HealthStatus == types.HealthStatusUnhealthy {
		return false
	}
	return true
}

func IsTaskStillHealthy(cfg aws.Config, cluster, taskId string) (bool, error) {
	client := ecs.NewFromConfig(cfg)
	return isTaskStillHealthyWithClient(context.Background(), cluster, taskId, client)
}

type ecsTaskDescriber2 interface {
	DescribeTasks(context.Context, *ecs.DescribeTasksInput, ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
}

func isTaskStillHealthyWithClient(ctx context.Context, cluster, taskId string, client ecsTaskDescriber2) (bool, error) {
	output, err := client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(cluster),
		Tasks:   []string{taskId},
	})
	if err != nil {
		return false, fmt.Errorf("failed to describe task: %w", err)
	}
	if len(output.Tasks) == 0 {
		return false, nil
	}
	return isTaskHealthy(output.Tasks[0]), nil
}

func MonitorTaskHealth(ctx context.Context, cfg aws.Config, cluster, taskId string, interval time.Duration, cancel context.CancelFunc) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			healthy, err := IsTaskStillHealthy(cfg, cluster, taskId)
			if err != nil || !healthy {
				cancel()
				return
			}
		}
	}
}

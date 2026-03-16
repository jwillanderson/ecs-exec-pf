package ecsexecpf

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockHealthChecker struct {
	tasks    []types.Task
	taskArns []string
}

func (m *mockHealthChecker) DescribeTasks(_ context.Context, input *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	return &ecs.DescribeTasksOutput{Tasks: m.tasks}, nil
}

func (m *mockHealthChecker) ListTasks(_ context.Context, _ *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	return &ecs.ListTasksOutput{TaskArns: m.taskArns}, nil
}

func TestPickHealthyTask_ReturnsFirstHealthy(t *testing.T) {
	mock := &mockHealthChecker{
		taskArns: []string{
			"arn:aws:ecs:us-east-1:123:task/cluster/task-1",
			"arn:aws:ecs:us-east-1:123:task/cluster/task-2",
		},
		tasks: []types.Task{
			{
				TaskArn:      aws.String("arn:aws:ecs:us-east-1:123:task/cluster/task-1"),
				LastStatus:   aws.String("STOPPED"),
				HealthStatus: types.HealthStatusHealthy,
			},
			{
				TaskArn:      aws.String("arn:aws:ecs:us-east-1:123:task/cluster/task-2"),
				LastStatus:   aws.String("RUNNING"),
				HealthStatus: types.HealthStatusHealthy,
			},
		},
	}

	taskId, err := pickHealthyTaskWithClient(context.Background(), "cluster", "svc", mock)
	require.NoError(t, err)
	assert.Equal(t, "task-2", taskId)
}

func TestPickHealthyTask_NoHealthyTasks(t *testing.T) {
	mock := &mockHealthChecker{
		taskArns: []string{"arn:aws:ecs:us-east-1:123:task/cluster/task-1"},
		tasks: []types.Task{
			{
				TaskArn:      aws.String("arn:aws:ecs:us-east-1:123:task/cluster/task-1"),
				LastStatus:   aws.String("RUNNING"),
				HealthStatus: types.HealthStatusUnhealthy,
			},
		},
	}

	_, err := pickHealthyTaskWithClient(context.Background(), "cluster", "svc", mock)
	require.ErrorContains(t, err, "no healthy tasks found")
}

func TestPickHealthyTask_NoTasks(t *testing.T) {
	mock := &mockHealthChecker{
		taskArns: []string{},
		tasks:    []types.Task{},
	}

	_, err := pickHealthyTaskWithClient(context.Background(), "cluster", "svc", mock)
	require.ErrorContains(t, err, "no running tasks found")
}

func TestIsTaskHealthy(t *testing.T) {
	tests := []struct {
		name     string
		task     types.Task
		expected bool
	}{
		{
			name:     "running and healthy",
			task:     types.Task{LastStatus: aws.String("RUNNING"), HealthStatus: types.HealthStatusHealthy},
			expected: true,
		},
		{
			name:     "running and unknown health",
			task:     types.Task{LastStatus: aws.String("RUNNING"), HealthStatus: types.HealthStatusUnknown},
			expected: true,
		},
		{
			name:     "running no health check",
			task:     types.Task{LastStatus: aws.String("RUNNING")},
			expected: true,
		},
		{
			name:     "stopped",
			task:     types.Task{LastStatus: aws.String("STOPPED"), HealthStatus: types.HealthStatusHealthy},
			expected: false,
		},
		{
			name:     "running but unhealthy",
			task:     types.Task{LastStatus: aws.String("RUNNING"), HealthStatus: types.HealthStatusUnhealthy},
			expected: false,
		},
		{
			name:     "nil status",
			task:     types.Task{},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, isTaskHealthy(tc.task))
		})
	}
}

func TestIsTaskStillHealthy_TaskGone(t *testing.T) {
	mock := &mockHealthChecker{tasks: []types.Task{}}

	healthy, err := isTaskStillHealthyWithClient(context.Background(), "cluster", "task-1", mock)
	require.NoError(t, err)
	assert.False(t, healthy)
}

func TestIsTaskStillHealthy_TaskHealthy(t *testing.T) {
	mock := &mockHealthChecker{
		tasks: []types.Task{
			{LastStatus: aws.String("RUNNING"), HealthStatus: types.HealthStatusHealthy},
		},
	}

	healthy, err := isTaskStillHealthyWithClient(context.Background(), "cluster", "task-1", mock)
	require.NoError(t, err)
	assert.True(t, healthy)
}

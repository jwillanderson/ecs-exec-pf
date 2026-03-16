package ecsexecpf

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTaskLister struct {
	Pages [][]string
	Err   error
}

func (m *mockTaskLister) ListTasks(_ context.Context, input *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	if m.Err != nil {
		return nil, m.Err
	}

	pageIndex := 0
	if input.NextToken != nil {
		fmt.Sscanf(*input.NextToken, "%d", &pageIndex)
	}

	output := &ecs.ListTasksOutput{
		TaskArns: m.Pages[pageIndex],
	}

	if pageIndex+1 < len(m.Pages) {
		next := fmt.Sprintf("%d", pageIndex+1)
		output.NextToken = &next
	}

	return output, nil
}

func TestListTasksForService(t *testing.T) {
	tests := []struct {
		name     string
		pages    [][]string
		err      error
		expected []string
		wantErr  string
	}{
		{
			name:     "single page",
			pages:    [][]string{{"arn:aws:ecs:us-east-1:123:task/cluster/task-abc", "arn:aws:ecs:us-east-1:123:task/cluster/task-def"}},
			expected: []string{"task-abc", "task-def"},
		},
		{
			name: "multiple pages",
			pages: [][]string{
				{"arn:aws:ecs:us-east-1:123:task/cluster/task-abc"},
				{"arn:aws:ecs:us-east-1:123:task/cluster/task-def"},
			},
			expected: []string{"task-abc", "task-def"},
		},
		{
			name:    "no tasks",
			pages:   [][]string{{}},
			wantErr: "no running tasks found",
		},
		{
			name:    "API error",
			pages:   [][]string{{}},
			err:     fmt.Errorf("access denied"),
			wantErr: "failed to list tasks",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockTaskLister{Pages: tc.pages, Err: tc.err}
			result, err := listTasksWithLister("test-cluster", "test-service", mock)

			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

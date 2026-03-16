package ecsexecpf

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockServiceLister struct {
	Pages [][]string
	Err   error
}

func (m *mockServiceLister) ListServices(_ context.Context, input *ecs.ListServicesInput, _ ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	if m.Err != nil {
		return nil, m.Err
	}

	pageIndex := 0
	if input.NextToken != nil {
		fmt.Sscanf(*input.NextToken, "%d", &pageIndex)
	}

	output := &ecs.ListServicesOutput{
		ServiceArns: m.Pages[pageIndex],
	}

	if pageIndex+1 < len(m.Pages) {
		next := fmt.Sprintf("%d", pageIndex+1)
		output.NextToken = &next
	}

	return output, nil
}

func TestListServices(t *testing.T) {
	tests := []struct {
		name     string
		pages    [][]string
		err      error
		expected []string
		wantErr  string
	}{
		{
			name:     "single page",
			pages:    [][]string{{"arn:aws:ecs:us-east-1:123:service/cluster/svc-b", "arn:aws:ecs:us-east-1:123:service/cluster/svc-a"}},
			expected: []string{"svc-a", "svc-b"},
		},
		{
			name: "multiple pages",
			pages: [][]string{
				{"arn:aws:ecs:us-east-1:123:service/cluster/svc-c"},
				{"arn:aws:ecs:us-east-1:123:service/cluster/svc-a"},
			},
			expected: []string{"svc-a", "svc-c"},
		},
		{
			name:    "no services",
			pages:   [][]string{{}},
			wantErr: "no services found",
		},
		{
			name:    "API error",
			pages:   [][]string{{}},
			err:     fmt.Errorf("access denied"),
			wantErr: "failed to list services",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockServiceLister{Pages: tc.pages, Err: tc.err}
			result, err := listServicesWithLister("test-cluster", mock)

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

func TestShortNameFromArn(t *testing.T) {
	tests := []struct {
		arn      string
		expected string
	}{
		{"arn:aws:ecs:us-east-1:123:service/cluster/my-service", "my-service"},
		{"arn:aws:ecs:us-east-1:123:task/abc123", "abc123"},
		{"arn:aws:ecs:us-east-1:123:task/cluster/abc123", "abc123"},
		{"simple-name", "simple-name"},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.expected, shortNameFromArn(tc.arn))
	}
}

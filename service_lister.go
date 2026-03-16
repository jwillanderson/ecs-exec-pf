package ecsexecpf

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

type ecsServiceLister interface {
	ListServices(context.Context, *ecs.ListServicesInput, ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
}

func ListServices(cfg aws.Config, cluster string) ([]string, error) {
	return listServicesWithLister(cluster, ecs.NewFromConfig(cfg))
}

func listServicesWithLister(cluster string, lister ecsServiceLister) ([]string, error) {
	var serviceNames []string
	var nextToken *string

	for {
		input := &ecs.ListServicesInput{
			Cluster:   aws.String(cluster),
			NextToken: nextToken,
		}

		output, err := lister.ListServices(context.Background(), input)
		if err != nil {
			return nil, fmt.Errorf("failed to list services for cluster %s: %w", cluster, err)
		}

		for _, arn := range output.ServiceArns {
			serviceNames = append(serviceNames, shortNameFromArn(arn))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	if len(serviceNames) == 0 {
		return nil, fmt.Errorf("no services found in cluster %s", cluster)
	}

	sort.Strings(serviceNames)
	return serviceNames, nil
}

func shortNameFromArn(arn string) string {
	parts := strings.Split(arn, "/")
	return parts[len(parts)-1]
}

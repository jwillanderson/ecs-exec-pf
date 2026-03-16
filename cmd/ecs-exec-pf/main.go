package main

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	ecsexecpf "github.com/jwillanderson/ecs-exec-pf"
	"golang.org/x/sync/errgroup"
)

func init() {
	log.SetFlags(0)
}

func main() {
	opts, err := parseArgs()
	if err != nil {
		log.Fatalf("failed to parse args: %s", err)
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("failed to load SDK config: %s", err)
	}

	var autoTask bool
	var service string

	if opts.Interactive {
		needPorts := !opts.PortsProvided
		result, err := ecsexecpf.SelectServiceAndTask(cfg, opts.Cluster, needPorts)
		if err != nil {
			log.Fatalf("interactive selection failed: %s", err)
		}
		opts.Task = result.Task
		autoTask = result.AutoTask
		service = result.Service

		if needPorts && len(result.PortMappings) > 0 {
			opts.Port = make([]int, len(result.PortMappings))
			opts.LocalPort = make([]int, len(result.PortMappings))
			for i, pm := range result.PortMappings {
				opts.Port[i] = pm.RemotePort
				opts.LocalPort[i] = pm.LocalPort
			}
		}
	}

	if len(opts.Port) == 0 || len(opts.LocalPort) == 0 {
		log.Fatal("port mappings are required")
	}

	if autoTask {
		runWithAutoReconnect(cfg, opts, service)
	} else {
		runOnce(cfg, opts)
	}
}

func runOnce(cfg aws.Config, opts *Options) {
	containerId, err := ecsexecpf.GetContainerId(cfg, opts.Cluster, opts.Task, opts.Container)
	if err != nil {
		log.Fatalf("failed to get container ID: %s", err)
	}

	errGroup, ctx := errgroup.WithContext(context.Background())
	for i := range opts.LocalPort {
		errGroup.Go(func() error {
			return ecsexecpf.StartSession(ctx, opts.Cluster, opts.Task, containerId, opts.Port[i], opts.LocalPort[i], opts.Debug)
		})
	}
	if err := errGroup.Wait(); err != nil {
		log.Fatalf("failed to start session: %s", err)
	}
}

func runWithAutoReconnect(cfg aws.Config, opts *Options, service string) {
	const healthCheckInterval = 10 * time.Second

	for {
		taskId, err := ecsexecpf.PickHealthyTask(cfg, opts.Cluster, service)
		if err != nil {
			log.Fatalf("failed to find healthy task: %s", err)
		}

		log.Printf("connecting to task %s", taskId)

		containerId, err := ecsexecpf.GetContainerId(cfg, opts.Cluster, taskId, opts.Container)
		if err != nil {
			log.Fatalf("failed to get container ID: %s", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		go ecsexecpf.MonitorTaskHealth(ctx, cfg, opts.Cluster, taskId, healthCheckInterval, cancel)

		errGroup, gctx := errgroup.WithContext(ctx)
		for i := range opts.LocalPort {
			errGroup.Go(func() error {
				return ecsexecpf.StartSession(gctx, opts.Cluster, taskId, containerId, opts.Port[i], opts.LocalPort[i], opts.Debug)
			})
		}

		err = errGroup.Wait()
		cancel()

		if ctx.Err() != nil {
			log.Printf("task %s became unhealthy, reconnecting...", taskId)
			continue
		}

		if err != nil {
			log.Fatalf("session failed: %s", err)
		}
		break
	}
}

# KubeSnooze

Because even clusters need a nap.

![KubeSnooze Logo](KubeSnooze.png)

KubeSnooze is a Kubernetes controller that watches specified deployments for their replica count. When all watched deployments are scaled to 0, it starts a configurable timer. After the timer expires, it sends the node to S3 sleep.

## Features

- Watch multiple deployments across different namespaces
- Configurable timer duration
- Simple environment variable based configuration
- Sleeps node all deployments scale to 0 and when timer expires

## Configuration

The controller is configured via environment variables:

- `WATCHED_DEPLOYMENTS`: Comma-separated list of deployments to watch in format `namespace/name`
  - Example: `default/app1,monitoring/grafana`
  - Required: Yes
- `TIMER_DURATION`: Duration to wait after all deployments are scaled to 0
  - Example: `1h`, `30m`, `2h30m`
  - Default: `1h`

## Installation

1. Label the node you want to be put to sleep:

```bash
kubectl label nodes <node-name> sleep-target=true
```

2. Deploy KubeSnooze:

```bash
kubectl apply -f k8s/kubesnooze.yaml
```

Ensure to modify the environment variables in `k8s/kubesnooze.yaml` to specify which deployments should be watched.

## How it Works

1. The controller runs on the node marked with `sleep-target=true`
2. It watches the specified deployments for replica changes
3. When all watched deployments have 0 replicas, it logs: "All watched deployments scaled to 0"
4. A timer starts with the configured duration
5. When the timer expires, it sends "mem" to `/sys/power/state` to suspend the machine
6. If any deployment scales up before the timer expires, the timer is cancelled

## License

Licensed under the Apache License, Version 2.0.

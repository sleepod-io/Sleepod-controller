# SleePod-controller

SleePod-controller is a Kubernetes controller designed to manage the sleep and wake cycles of your Kubernetes workloads. It allows you to define policies for when your deployments and statefulsets should be scaled down (sleep) and scaled up (wake), helping you save resources and costs in non-production environments.

## How it Works

SleePod-controller operates by managing `SleepPolicy` resources. Each policy defines a schedule for scaling deployments and statefulsets. The controller calculates the next sleep or wake time based on your configuration and the specified timezone.

When a scheduled time is reached, the controller creates internal `SleepOrder` resources to execute the scaling operations (Sleep: scale to 0, Wake: scale to original replicas).

```mermaid
graph TD
    User[User] -->|Creates| Policy[SleepPolicy CR]
    Policy -->|Watches| Controller[SleePod Controller]
    Controller -->|Calculates Schedule| Timer{Time Check}
    Timer -->|Sleep Time| SleepCtx[Create SleepOrder (Sleep)]
    Timer -->|Wake Time| WakeCtx[Create SleepOrder (Wake)]
    SleepCtx -->|Scale Down| Resources[Deployments/StatefulSets]
    WakeCtx -->|Scale Up| Resources
```

> **Note**: Users only need to define `SleepPolicy`. The `SleepOrder` resources are created and managed automatically by the controller. You do not need to create them manually.

## Features

- **Automated Sleep/Wake Schedules**: Define precise schedules for sleeping and waking workloads.
- **Policy-Based Management**: Create `SleepPolicy` resources to group and manage workloads.
- **Resource Support**: Supports Deployments and StatefulSets.
- **Configurable Timezones**: Schedule sleep/wake times in your local timezone.

## Installation

### Prerequisites

- Kubernetes cluster (v1.24+)
- Helm (3.0+)

### Install using Helm

1. Add the Helm repository:
   ```bash
   helm repo add sleepod https://shaygef123.github.io/SleePod-controller
   helm repo update
   ```

2. Install the Helm chart:
   ```bash
   helm install sleepod-controller sleepod/sleepod-controller -n sleepod-system --create-namespace
   ```

3. Verify the installation:
   ```bash
   kubectl get pods -n sleepod-system
   ```

## Usage

### 1. Create a SleepPolicy

Create a `SleepPolicy` manifest to define your sleep schedule. You can define default settings for all deployments/statefulsets in a namespace, or target specific ones by name.

```yaml
apiVersion: sleepod.shaygef.io/v1alpha1
kind: SleepPolicy
metadata:
  name: dev-team-policy
  namespace: default
spec:
  timezone: "UTC"
  deployments:
    # "default" applies to all deployments in the namespace unless overridden
    default:
      enable: true
      sleepAt: "20:00" # 8:00 PM
      wakeAt: "08:00"  # 8:00 AM
    # specific deployment override
    backend-service:
      enable: false # Do not sleep this service
  statefulsets:
    db-primary:
      enable: true
      sleepAt: "22:00"
      wakeAt: "06:00"
```

### 2. Apply the Policy

```bash
kubectl apply -f my-policy.yaml
```

The controller will now monitor the resources in the `default` namespace. It will automatically create `SleepOrder` resources to scale the workloads down and up at the specified times.

## Configuration

The Helm chart can be customized using `values.yaml`. Here are the most common configurations:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.defaultTimezone` | Default timezone for policies if not specified | `"UTC"` |
| `config.defaultSleepAt` | Default sleep time | `"20:00"` |
| `config.defaultWakeAt` | Default wake time | `"08:00"` |
| `config.excludedNamespaces` | List of namespaces the controller should ignore | `kube-system`, etc. |
| `config.namespaceDelaySeconds`| Delay before processing changes | `20` |
| `controllerManager.replicas` | Number of controller replicas | `1` |
| `resources.requests/limits` | CPU/Memory requests and limits | (See values.yaml) |


## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details on how to get started.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Contact

**Shay Geffen**

- Email: shaygef123@gmail.com
- LinkedIn: [Shay Geffen](https://www.linkedin.com/in/shay-geffen-051357217/)
- Github [shaygef123](https://github.com/shaygef123)

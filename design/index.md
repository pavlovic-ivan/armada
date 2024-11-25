# System overview

This document is meant to be an overview of Armada for new users. We cover the architecture of Armada, show how jobs are represented, and explain how jobs are queued and scheduled.

If you just want to learn how to submit jobs to Armada, see:

- [User guide](../user.md)

If you want to see a quick overview of Armadas components, see:

- [Relationships diagram](./relationships_diagram.md)

## Architecture

Armada consists of two main components:
- The Armada server, which is responsible for accepting jobs from users and deciding in what order, and on which Kubernetes cluster, jobs should run. Users submit jobs to the Armada server through the `armadactl` command-line utility or via a gRPC or REST API.
- The Armada executor, of which there is one instance running in each Kubernetes cluster that Armada is connected to. Each Armada executor instance regularly notifies the server of how much spare capacity it has available and requests jobs to run. Users of Armada never interact with the executor directly.

All state relating to the Armada server is stored in [Redis](https://redis.io/), which may use replication combined with failover for redundancy. Hence, the Armada server is itself stateless and is easily replicated by running multiple independent instances. Both the server and the executors are intended to be run in Kubernetes pods. We show a diagram of the architecture below.

![How Armada works](../assets/img/batch-api.svg)

### Job leasing

To avoid jobs being lost if a cluster or its executor becomes unavailable, each job assigned to an executor has an associated timeout. Armada executors are required to check in with the server regularly and if an executor responsible for running a particular job fails to check in within that timeout, the server will re-schedule the job on another cluster.

## Jobs and job sets

A job is the most basic unit of work in Armada, and is represented by a Kubernetes pod specification (podspec) with additional metadata specific to Armada. Armada handles creating, running, and removing containers as necessary for each job. Hence, Armada is essentially a system for managing the life cycle of a set of containerised applications representing a batch job.

The Armada workflow is:

1. Create a job specification, which is a Kubernetes podspec with a few additional metadata fields.
2. Submit the job specification to one of Armada's job queues using the `armadactl` CLI utility or through the Armada gRPC or REST API.

For example, a job that sleeps for 60 seconds could be represented by the following yaml file.

```yaml
queue: test
jobSetId: set1
jobs:
  - priority: 0
    podSpecs:
      - terminationGracePeriodSeconds: 0
        restartPolicy: Never
        containers:
          - name: sleep
            imagePullPolicy: IfNotPresent
            image: busybox:latest
            args:
              - sleep
              - 60s
            resources:
              limits:
                memory: 64Mi
                cpu: 150m
              requests:
                memory: 64Mi
                cpu: 150m
```

In the above yaml snippet, `podSpec` is a Kubernetes podspec, which consists of one or more containers that contain the user code to be run. In addition, the job specification (jobspec) contains metadata fields specific to Armada:

- `queue`: which of the available job queues the job should be submitted to.
- `priority`: the job priority (lower values indicate higher priority).
- `jobSetId`: jobs with the same `jobSetId` can be followed and cancelled in a single operation. The `jobSetId` has no impact on scheduling.

Queues and scheduling is explained in more detail below.

For more examples, see the [user guide](../user.md).

### Job events

A job event is generated whenever the state of a job changes (e.g., when changing from submitted to running or from running to completed) and is a timestamped message containing event-specific information (e.g., an exit code for a completed job). All events generated by jobs part of the same job set are grouped together and published via a [Redis stream](https://redis.io/topics/streams-intro). There are unique streams for each job set to facilitate subscribing only to events generated by jobs in a particular set, which can be done via the Armada API.

Armada records all events necessary to reconstruct the state of each job and, after a job has been completed,  the only information retained about the job is the events generated by it.
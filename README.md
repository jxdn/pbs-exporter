# PBS Exporter

A Prometheus exporter for PBS (Portable Batch System) cluster monitoring.

## Features

- **Job Metrics**: Track running jobs by user, queue, and status
- **Node Metrics**: Monitor node states, CPU/GPU usage, and memory utilization
- **Queue Metrics**: Every queue PBS reports (from `qstat -Qf -F json`), so new queues appear automatically
- **Real-time Updates**: Metrics are updated every 60 seconds

## Architecture

The application is structured into several packages for better maintainability:

### `internal/metrics`
Contains all Prometheus metrics definitions and registry management:
- Job-related metrics (running jobs by user/queue, total jobs by status)
- Node-related metrics (state, CPU/GPU/memory usage)
- Node count metrics (free, job-busy, offline, down nodes)

### `internal/pbs`
Handles PBS command execution and data parsing:
- `Client`: Executes PBS commands (`qstat`, `pbsnodes`)
- `JobData`: Structured representation of job information
- `NodeData`: Structured representation of node information
- Parsing utilities for PBS output formats

### `internal/server`
Coordinates the HTTP server and metrics updates:
- `Server`: Manages the overall application state
- Metrics update coordination
- Data flow between PBS client and metrics registry

### `main.go`
Entry point that orchestrates all components:
- Initializes all packages
- Starts background metrics collection
- Runs the HTTP server

## Metrics

### Job Metrics
- `qstat_running_jobs_by_user`: Number of running jobs per user
- `qstat_running_jobs_by_queue`: Number of running jobs per queue
- `qstat_jobs_in_queue`: Total number of jobs in each queue
- `qstat_total_running_jobs`: Total number of running jobs
- `qstat_total_r_jobs`: Total Running (R) jobs
- `qstat_total_h_jobs`: Total Hold (H) jobs
- `qstat_total_f_jobs`: Total Finished (F) jobs
- `qstat_total_q_jobs`: Total Queuing (Q) jobs
- `qstat_total_e_jobs`: Total Error (E) jobs
- `qstat_total_b_jobs`: Total Array Job Running (B) jobs
- `qstat_total_all_jobs`: Total number of all jobs
- `qstat_jobs_by_status`: Number of jobs by status

### Per-Job Metrics (from `qstat -f -F json -t`)
Running jobs:
- `pbs_job_info{job_id,user,queue,project,job_name,node}`: Always 1, one series per job per node (full job name)
- `pbs_job_gpus{job_id,node}`: GPUs allocated to the job on that node
- `pbs_job_walltime_used_seconds{job_id}`: Walltime used
- `pbs_job_walltime_requested_seconds{job_id}`: Walltime requested

Queued (Q) and held (H) jobs:
- `pbs_job_queued_info{job_id,user,queue,project,job_name,state}`: Always 1
- `pbs_job_queued_seconds{job_id}`: Seconds since the job entered the queue (`qtime`)

Per project (`project` is `none` when a job has no project):
- `pbs_project_jobs{project,state}`: Jobs per project in state R, Q or H
- `pbs_project_gpus{project,state}`: GPUs allocated (R) or requested (Q, H) per project

### Node Metrics
- `pbs_node_state`: Node state (1=free, 2=job-busy, 3=offline, 4=down)
- `pbs_node_jobs`: Number of jobs on node
- `pbs_node_cpus_available`: Available CPUs on node
- `pbs_node_cpus_used`: Used CPUs on node
- `pbs_node_cpus_total`: Total CPUs on node
- `pbs_node_gpus_available`: Available GPUs on node
- `pbs_node_gpus_used`: Used GPUs on node
- `pbs_node_gpus_total`: Total GPUs on node
- `pbs_node_memory_available_gb`: Available memory on node in GB
- `pbs_node_memory_used_gb`: Used memory on node in GB
- `pbs_node_memory_total_gb`: Total memory on node in GB

### Queue Metrics (from `qstat -Qf -F json`, every queue PBS reports)
- `pbs_queue_info{queue,type}`: Always 1; `type` is `Execution` or `Route`
- `pbs_queue_jobs{queue,state}`: Jobs per queue and state (`Queued`, `Held`, `Waiting`, `Running`, `Exiting`, `Begun`, `Transit`)
- `pbs_queue_running_jobs{queue}`, `pbs_queue_queued_jobs{queue}`: Running and queued (`Q`) jobs
- `pbs_queue_enabled{queue}`, `pbs_queue_started{queue}`: 1 or 0
- `pbs_queue_walltime_seconds{queue}`: Maximum walltime (`resources_max.walltime`), only for queues that set one
- `qstatq_total_running`, `qstatq_total_queued`: Totals over all queues

### Exporter Health Metrics
Per PBS command (`command` = `qstat_json`, `pbsnodes`, `qstat_queues`, `qstat_bf`):
- `pbs_exporter_command_success{command}`: 1 if the last run succeeded, 0 if it failed or timed out
- `pbs_exporter_command_duration_seconds{command}`: How long the last run took
- `pbs_exporter_last_success_timestamp_seconds{command}`: Unix time of the last successful run

Example alert: `time() - pbs_exporter_last_success_timestamp_seconds{command="qstat_json"} > 600` (no fresh job data for 10 minutes).

### Node Count Metrics
- `pbs_node_count_free`: Number of nodes in free state
- `pbs_node_count_busy`: Number of nodes in job-busy state
- `pbs_node_count_offline`: Number of nodes in offline state
- `pbs_node_count_down`: Number of nodes in down state

## Grafana Dashboard

[`grafana/pbs-cluster-dashboard.json`](grafana/pbs-cluster-dashboard.json) is a ready-to-import dashboard. Import it with **Dashboards → New → Import → Upload dashboard JSON file**, then pick your Prometheus data source and a cluster at the top.

It starts with an **executive summary**: GPU allocation and actual GPU use, GPUs held but idle, running and waiting jobs, GPU demand waiting, longest queue wait, nodes by state, active projects and exporter health. Below it are rows for trends, projects, users, queues, idle GPU jobs, longest-waiting jobs, nodes and exporter health.

The **Cluster** dropdown lists every scrape job that exports `pbs_node_state`, so each pbs-exporter in the same Prometheus appears automatically. The actual-GPU-use panels match DCGM GPUs to PBS nodes by name, and the idle panels need the rules in `prometheus/` to be loaded.

## Idle GPU Jobs

[`prometheus/idle_gpu_jobs.rules.yml`](prometheus/idle_gpu_jobs.rules.yml) finds running jobs that hold GPUs but don't use them. It joins `pbs_job_*` with the per-GPU `gpu_process_count{exported_job="<job id>"}` metric from the compute nodes and with `DCGM_FI_DEV_GPU_UTIL`. A job is idle when it has run for over 30 minutes and either none of its GPUs has a process or its GPUs averaged under 1% util over 30 minutes.

Add the file to `rule_files` in `prometheus.yml`. For a Grafana table, use the instant query `pbs:job_gpu_idle` with Format = Table. Its columns are user, job_name, job_id, node and value (GPUs held). The `PBSJobGPUIdle` alert fires after a job has been idle for 30 more minutes.

Only nodes that report `gpu_process_count` are checked. PBS records which node a job runs on and how many GPUs it has, but not which GPU, so the link from GPU to job comes from `exported_job`.

## Usage

1. Build the application:
   ```bash
   go build -o pbs-exporter
   ```

2. Run the exporter:
   ```bash
   ./pbs-exporter
   ```

3. Access metrics at `http://localhost:8888/metrics`

## Configuration

| Flag | Default | Meaning |
|---|---|---|
| `-listen` | `0.0.0.0:8888` | Address to serve metrics on |
| `-interval` | `60s` | How often to query PBS |
| `-command-timeout` | `45s` | Time limit for each PBS command; a stuck command is killed and retried next cycle |

Each cycle runs 4 PBS commands: `qstat -f -F json -t`, `pbsnodes -aSj`, `qstat -Qf -F json` and `qstat -Bf`. Metrics are only replaced after a command succeeds, so a failing or slow PBS server leaves the last good values in place instead of empty metrics. Watch `pbs_exporter_command_success` to catch that.

## Dependencies

- Go 1.21+
- Prometheus client library
- PBS commands (`qstat`, `pbsnodes`) must be available in PATH

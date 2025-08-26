# PBS Exporter

A Prometheus exporter for PBS (Portable Batch System) cluster monitoring.

![PBS Cluster Dashboard](docs/pbs-dashboard.png)

*Real-time PBS cluster monitoring dashboard showing job status, node availability, and resource utilization*

## Features

- **Job Metrics**: Track running jobs by user, queue, and status
- **Node Metrics**: Monitor node states, CPU/GPU usage, and memory utilization
- **Queue Metrics**: Track job distribution across different queues
- **Real-time Updates**: Metrics are updated every 60 seconds
- **Dashboard Integration**: Compatible with Grafana and other monitoring dashboards

## Architecture

The application is structured into several packages for better maintainability:

### `internal/metrics`
Contains all Prometheus metrics definitions and registry management:
- Job-related metrics (running jobs by user/queue, total jobs by status)
- Node-related metrics (state, CPU/GPU/memory usage)
- Node count metrics (free, busy, offline, down nodes)

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

### Node Metrics
- `pbs_node_state`: Node state (1=free, 2=busy, 3=offline, 4=down)
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

### Node Count Metrics
- `pbs_node_count_free`: Number of nodes in free state
- `pbs_node_count_busy`: Number of nodes in busy state
- `pbs_node_count_offline`: Number of nodes in offline state
- `pbs_node_count_down`: Number of nodes in down state

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

The application runs on port 8888 by default and updates metrics every 60 seconds. These values can be modified in the `main.go` file.

## Dependencies

- Go 1.21+
- Prometheus client library
- PBS commands (`qstat`, `pbsnodes`) must be available in PATH

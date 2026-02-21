package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Registry struct {
	RunningJobsByUser  *prometheus.GaugeVec
	QueuedJobsByUser   *prometheus.GaugeVec
	RunningJobsByQueue *prometheus.GaugeVec
	JobsInQueue        *prometheus.GaugeVec
	TotalRunningJobs   prometheus.Gauge
	TotalRJobs         prometheus.Gauge
	TotalHJobs         prometheus.Gauge
	TotalFJobs         prometheus.Gauge
	TotalQJobs         prometheus.Gauge
	TotalEJobs         prometheus.Gauge
	TotalBJobs         prometheus.Gauge
	TotalAllJobs       prometheus.Gauge
	JobsByStatus       *prometheus.GaugeVec

	NodeState           *prometheus.GaugeVec
	NodeJobs            *prometheus.GaugeVec
	NodeCpusAvailable   *prometheus.GaugeVec
	NodeCpusUsed        *prometheus.GaugeVec
	NodeCpusTotal       *prometheus.GaugeVec
	NodeGpusAvailable   *prometheus.GaugeVec
	NodeGpusUsed        *prometheus.GaugeVec
	NodeGpusTotal       *prometheus.GaugeVec
	NodeMemoryAvailable *prometheus.GaugeVec
	NodeMemoryUsed      *prometheus.GaugeVec
	NodeMemoryTotal     *prometheus.GaugeVec

	NodeCpuUtilization    *prometheus.GaugeVec
	NodeGpuUtilization    *prometheus.GaugeVec
	NodeMemoryUtilization *prometheus.GaugeVec

	NodeCountFree    prometheus.Gauge
	NodeCountBusy    prometheus.Gauge
	NodeCountOffline prometheus.Gauge
	NodeCountDown    prometheus.Gauge

	ClusterCpusTotal         prometheus.Gauge
	ClusterCpusAvailable     prometheus.Gauge
	ClusterCpusUsed          prometheus.Gauge
	ClusterGpusTotal         prometheus.Gauge
	ClusterGpusAvailable     prometheus.Gauge
	ClusterGpusUsed          prometheus.Gauge
	ClusterMemoryTotalGb     prometheus.Gauge
	ClusterMemoryAvailableGb prometheus.Gauge
	ClusterMemoryUsedGb      prometheus.Gauge

	QueueSummaryRunning prometheus.Gauge
	QueueSummaryQueued  prometheus.Gauge
	QueueQueuedByQueue  *prometheus.GaugeVec
	QueueEnabled        *prometheus.GaugeVec
	QueueStarted        *prometheus.GaugeVec
	QueueRunningJobs    *prometheus.GaugeVec
	QueueQueuedJobs     *prometheus.GaugeVec
	QueueWalltime       *prometheus.GaugeVec

	ServerState              prometheus.Gauge
	ServerSchedulingEnabled  prometheus.Gauge
	ServerTotalJobs          prometheus.Gauge
	ServerJobsRunning        prometheus.Gauge
	ServerJobsQueued         prometheus.Gauge
	ServerJobsHeld           prometheus.Gauge
	ServerJobsWaiting        prometheus.Gauge
	ServerJobsExiting        prometheus.Gauge
	ServerResourcesNcpus     prometheus.Gauge
	ServerResourcesMemGb     prometheus.Gauge
	ServerResourcesNodect    prometheus.Gauge
	ServerLicensesAvailable  prometheus.Gauge
	ServerLicensesUsed       prometheus.Gauge
	ServerMaxArraySize       prometheus.Gauge
	ServerJobHistoryEnabled  prometheus.Gauge
	ServerJobHistoryDuration prometheus.Gauge
	PBSVersion               *prometheus.GaugeVec

	registry *prometheus.Registry
}

func NewRegistry() *Registry {
	r := &Registry{
		RunningJobsByUser: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "qstat_running_jobs_by_user",
				Help: "Number of running jobs per user",
			},
			[]string{"user"},
		),

		QueuedJobsByUser: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "qstat_queued_jobs_by_user",
				Help: "Number of queued jobs per user",
			},
			[]string{"user"},
		),

		RunningJobsByQueue: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "qstat_running_jobs_by_queue",
				Help: "Number of running jobs per queue",
			},
			[]string{"queue"},
		),

		JobsInQueue: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "qstat_jobs_in_queue",
				Help: "Total number of jobs in each queue",
			},
			[]string{"queue"},
		),

		TotalRunningJobs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "qstat_total_running_jobs",
				Help: "Total number of running jobs",
			},
		),

		TotalRJobs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "qstat_total_r_jobs",
				Help: "Total number of Running (R) jobs",
			},
		),

		TotalHJobs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "qstat_total_h_jobs",
				Help: "Total number of Hold (H) jobs",
			},
		),

		TotalFJobs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "qstat_total_f_jobs",
				Help: "Total number of Finished (F) jobs",
			},
		),

		TotalQJobs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "qstat_total_q_jobs",
				Help: "Total number of Queuing (Q) jobs",
			},
		),

		TotalEJobs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "qstat_total_e_jobs",
				Help: "Total number of Error (E) jobs",
			},
		),

		TotalBJobs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "qstat_total_b_jobs",
				Help: "Total number of Array Job Running (B) jobs",
			},
		),

		TotalAllJobs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "qstat_total_all_jobs",
				Help: "Total number of all jobs",
			},
		),

		JobsByStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "qstat_jobs_by_status",
				Help: "Number of jobs by status",
			},
			[]string{"status"},
		),

		NodeState: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_node_state",
				Help: "Node state (1=free, 2=job-busy, 3=offline, 4=down)",
			},
			[]string{"node"},
		),

		NodeJobs: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_node_jobs",
				Help: "Number of jobs on node",
			},
			[]string{"node"},
		),

		NodeCpusAvailable: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_node_cpus_available",
				Help: "Available CPUs on node",
			},
			[]string{"node"},
		),

		NodeCpusUsed: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_node_cpus_used",
				Help: "Used CPUs on node",
			},
			[]string{"node"},
		),

		NodeCpusTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_node_cpus_total",
				Help: "Total CPUs on node",
			},
			[]string{"node"},
		),

		NodeGpusAvailable: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_node_gpus_available",
				Help: "Available GPUs on node",
			},
			[]string{"node"},
		),

		NodeGpusUsed: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_node_gpus_used",
				Help: "Used GPUs on node",
			},
			[]string{"node"},
		),

		NodeGpusTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_node_gpus_total",
				Help: "Total GPUs on node",
			},
			[]string{"node"},
		),

		NodeMemoryAvailable: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_node_memory_available_gb",
				Help: "Available memory on node in GB",
			},
			[]string{"node"},
		),

		NodeMemoryUsed: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_node_memory_used_gb",
				Help: "Used memory on node in GB",
			},
			[]string{"node"},
		),

		NodeMemoryTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_node_memory_total_gb",
				Help: "Total memory on node in GB",
			},
			[]string{"node"},
		),

		NodeCpuUtilization: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_node_cpu_utilization",
				Help: "CPU utilization percentage on node",
			},
			[]string{"node"},
		),

		NodeGpuUtilization: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_node_gpu_utilization",
				Help: "GPU utilization percentage on node",
			},
			[]string{"node"},
		),

		NodeMemoryUtilization: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_node_memory_utilization",
				Help: "Memory utilization percentage on node",
			},
			[]string{"node"},
		),

		NodeCountFree: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_node_count_free",
				Help: "Number of nodes in free state",
			},
		),

		NodeCountBusy: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_node_count_busy",
				Help: "Number of nodes in job-busy state",
			},
		),

		NodeCountOffline: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_node_count_offline",
				Help: "Number of nodes in offline state",
			},
		),

		NodeCountDown: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_node_count_down",
				Help: "Number of nodes in down state",
			},
		),

		ClusterCpusTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_cluster_cpus_total",
				Help: "Total CPUs in cluster",
			},
		),

		ClusterCpusAvailable: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_cluster_cpus_available",
				Help: "Available CPUs in cluster",
			},
		),

		ClusterCpusUsed: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_cluster_cpus_used",
				Help: "Used CPUs in cluster",
			},
		),

		ClusterGpusTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_cluster_gpus_total",
				Help: "Total GPUs in cluster",
			},
		),

		ClusterGpusAvailable: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_cluster_gpus_available",
				Help: "Available GPUs in cluster",
			},
		),

		ClusterGpusUsed: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_cluster_gpus_used",
				Help: "Used GPUs in cluster",
			},
		),

		ClusterMemoryTotalGb: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_cluster_memory_total_gb",
				Help: "Total memory in cluster in GB",
			},
		),

		ClusterMemoryAvailableGb: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_cluster_memory_available_gb",
				Help: "Available memory in cluster in GB",
			},
		),

		ClusterMemoryUsedGb: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_cluster_memory_used_gb",
				Help: "Used memory in cluster in GB",
			},
		),

		QueueSummaryRunning: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "qstatq_total_running",
				Help: "Total running jobs from qstat -q summary",
			},
		),

		QueueSummaryQueued: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "qstatq_total_queued",
				Help: "Total queued jobs from qstat -q summary",
			},
		),

		QueueQueuedByQueue: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "qstat_que_by_queue",
				Help: "Queued jobs per queue from qstat -q",
			},
			[]string{"queue"},
		),

		QueueEnabled: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_queue_enabled",
				Help: "Whether queue is enabled (1=enabled, 0=disabled)",
			},
			[]string{"queue"},
		),

		QueueStarted: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_queue_started",
				Help: "Whether queue is started (1=started, 0=stopped)",
			},
			[]string{"queue"},
		),

		QueueRunningJobs: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_queue_running_jobs",
				Help: "Running jobs per queue",
			},
			[]string{"queue"},
		),

		QueueQueuedJobs: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_queue_queued_jobs",
				Help: "Queued jobs per queue",
			},
			[]string{"queue"},
		),

		QueueWalltime: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_queue_walltime_seconds",
				Help: "Maximum walltime per queue in seconds",
			},
			[]string{"queue"},
		),

		ServerState: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_server_state",
				Help: "Server state (1=Active, 0=Inactive)",
			},
		),

		ServerSchedulingEnabled: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_server_scheduling_enabled",
				Help: "Whether scheduling is enabled (1=True, 0=False)",
			},
		),

		ServerTotalJobs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_server_total_jobs",
				Help: "Total jobs tracked by server",
			},
		),

		ServerJobsRunning: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_server_jobs_running",
				Help: "Running jobs from server state_count",
			},
		),

		ServerJobsQueued: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_server_jobs_queued",
				Help: "Queued jobs from server state_count",
			},
		),

		ServerJobsHeld: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_server_jobs_held",
				Help: "Held jobs from server state_count",
			},
		),

		ServerJobsWaiting: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_server_jobs_waiting",
				Help: "Waiting jobs from server state_count",
			},
		),

		ServerJobsExiting: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_server_jobs_exiting",
				Help: "Exiting jobs from server state_count",
			},
		),

		ServerResourcesNcpus: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_server_resources_ncpus",
				Help: "Total CPUs assigned by server",
			},
		),

		ServerResourcesMemGb: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_server_resources_mem_gb",
				Help: "Total memory assigned by server in GB",
			},
		),

		ServerResourcesNodect: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_server_resources_nodect",
				Help: "Number of nodes with assigned resources",
			},
		),

		ServerLicensesAvailable: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_server_licenses_available",
				Help: "Available PBS licenses",
			},
		),

		ServerLicensesUsed: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_server_licenses_used",
				Help: "Used PBS licenses",
			},
		),

		ServerMaxArraySize: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_server_max_array_size",
				Help: "Maximum array job size",
			},
		),

		ServerJobHistoryEnabled: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_server_job_history_enabled",
				Help: "Whether job history is enabled (1=True, 0=False)",
			},
		),

		ServerJobHistoryDuration: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_server_job_history_duration_hours",
				Help: "Job history duration in hours",
			},
		),

		PBSVersion: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_version_info",
				Help: "PBS version information",
			},
			[]string{"version"},
		),

		registry: prometheus.NewRegistry(),
	}

	r.registerMetrics()
	return r
}

func (r *Registry) registerMetrics() {
	r.registry.MustRegister(
		r.RunningJobsByUser,
		r.QueuedJobsByUser,
		r.RunningJobsByQueue,
		r.JobsInQueue,
		r.TotalRunningJobs,
		r.TotalRJobs,
		r.TotalHJobs,
		r.TotalFJobs,
		r.TotalQJobs,
		r.TotalEJobs,
		r.TotalBJobs,
		r.TotalAllJobs,
		r.JobsByStatus,
		r.NodeState,
		r.NodeJobs,
		r.NodeCpusAvailable,
		r.NodeCpusUsed,
		r.NodeCpusTotal,
		r.NodeGpusAvailable,
		r.NodeGpusUsed,
		r.NodeGpusTotal,
		r.NodeMemoryAvailable,
		r.NodeMemoryUsed,
		r.NodeMemoryTotal,
		r.NodeCpuUtilization,
		r.NodeGpuUtilization,
		r.NodeMemoryUtilization,
		r.NodeCountFree,
		r.NodeCountBusy,
		r.NodeCountOffline,
		r.NodeCountDown,
		r.ClusterCpusTotal,
		r.ClusterCpusAvailable,
		r.ClusterCpusUsed,
		r.ClusterGpusTotal,
		r.ClusterGpusAvailable,
		r.ClusterGpusUsed,
		r.ClusterMemoryTotalGb,
		r.ClusterMemoryAvailableGb,
		r.ClusterMemoryUsedGb,
		r.QueueSummaryRunning,
		r.QueueSummaryQueued,
		r.QueueQueuedByQueue,
		r.QueueEnabled,
		r.QueueStarted,
		r.QueueRunningJobs,
		r.QueueQueuedJobs,
		r.QueueWalltime,
		r.ServerState,
		r.ServerSchedulingEnabled,
		r.ServerTotalJobs,
		r.ServerJobsRunning,
		r.ServerJobsQueued,
		r.ServerJobsHeld,
		r.ServerJobsWaiting,
		r.ServerJobsExiting,
		r.ServerResourcesNcpus,
		r.ServerResourcesMemGb,
		r.ServerResourcesNodect,
		r.ServerLicensesAvailable,
		r.ServerLicensesUsed,
		r.ServerMaxArraySize,
		r.ServerJobHistoryEnabled,
		r.ServerJobHistoryDuration,
		r.PBSVersion,
	)
}

func (r *Registry) GetRegistry() *prometheus.Registry {
	return r.registry
}

func (r *Registry) ResetJobMetrics() {
	r.RunningJobsByUser.Reset()
	r.QueuedJobsByUser.Reset()
	r.RunningJobsByQueue.Reset()
	r.JobsInQueue.Reset()
	r.JobsByStatus.Reset()
}

func (r *Registry) ResetNodeMetrics() {
	r.NodeState.Reset()
	r.NodeJobs.Reset()
	r.NodeCpusAvailable.Reset()
	r.NodeCpusUsed.Reset()
	r.NodeCpusTotal.Reset()
	r.NodeGpusAvailable.Reset()
	r.NodeGpusUsed.Reset()
	r.NodeGpusTotal.Reset()
	r.NodeMemoryAvailable.Reset()
	r.NodeMemoryUsed.Reset()
	r.NodeMemoryTotal.Reset()
	r.NodeCpuUtilization.Reset()
	r.NodeGpuUtilization.Reset()
	r.NodeMemoryUtilization.Reset()
}

func (r *Registry) ResetQueueMetrics() {
	r.QueueEnabled.Reset()
	r.QueueStarted.Reset()
	r.QueueRunningJobs.Reset()
	r.QueueQueuedJobs.Reset()
	r.QueueWalltime.Reset()
	r.QueueQueuedByQueue.Reset()
}

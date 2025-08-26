package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Registry holds all Prometheus metrics for the PBS exporter
type Registry struct {
	// Job metrics
	RunningJobsByUser  *prometheus.GaugeVec
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

	// Node metrics
	NodeState            *prometheus.GaugeVec
	NodeJobs             *prometheus.GaugeVec
	NodeCpusAvailable    *prometheus.GaugeVec
	NodeCpusUsed         *prometheus.GaugeVec
	NodeCpusTotal        *prometheus.GaugeVec
	NodeGpusAvailable    *prometheus.GaugeVec
	NodeGpusUsed         *prometheus.GaugeVec
	NodeGpusTotal        *prometheus.GaugeVec
	NodeMemoryAvailable  *prometheus.GaugeVec
	NodeMemoryUsed       *prometheus.GaugeVec
	NodeMemoryTotal      *prometheus.GaugeVec

	// Node count metrics
	NodeCountFree    prometheus.Gauge
	NodeCountBusy    prometheus.Gauge
	NodeCountOffline prometheus.Gauge
	NodeCountDown    prometheus.Gauge

	// qstat -q summary totals
	QueueSummaryRunning prometheus.Gauge
	QueueSummaryQueued  prometheus.Gauge
	QueueQueuedByQueue  *prometheus.GaugeVec

	// Prometheus registry
	registry *prometheus.Registry
}

// NewRegistry creates and returns a new metrics registry
func NewRegistry() *Registry {
	r := &Registry{
		// Job metrics
		RunningJobsByUser: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "qstat_running_jobs_by_user",
				Help: "Number of running jobs per user",
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

		// Node metrics
		NodeState: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pbs_node_state",
				Help: "Node state (1=free, 2=busy, 3=offline, 4=down)",
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

		// Node count metrics
		NodeCountFree: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_node_count_free",
				Help: "Number of nodes in free state (status=1)",
			},
		),

		NodeCountBusy: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_node_count_busy",
				Help: "Number of nodes in busy state (status=2)",
			},
		),

		NodeCountOffline: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_node_count_offline",
				Help: "Number of nodes in offline state (status=3)",
			},
		),

		NodeCountDown: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pbs_node_count_down",
				Help: "Number of nodes in down state (status=4)",
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

		registry: prometheus.NewRegistry(),
	}

	// Register all metrics
	r.registerMetrics()

	return r
}

// registerMetrics registers all metrics with the Prometheus registry
func (r *Registry) registerMetrics() {
	r.registry.MustRegister(
		r.RunningJobsByUser,
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
		r.NodeCountFree,
		r.NodeCountBusy,
		r.NodeCountOffline,
		r.NodeCountDown,
		r.QueueSummaryRunning,
		r.QueueSummaryQueued,
		r.QueueQueuedByQueue,
	)
}

// GetRegistry returns the underlying Prometheus registry
func (r *Registry) GetRegistry() *prometheus.Registry {
	return r.registry
}

// ResetJobMetrics resets all job-related metrics
func (r *Registry) ResetJobMetrics() {
	r.RunningJobsByUser.Reset()
	r.RunningJobsByQueue.Reset()
	r.JobsInQueue.Reset()
	r.JobsByStatus.Reset()
}

// ResetNodeMetrics resets all node-related metrics
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
}

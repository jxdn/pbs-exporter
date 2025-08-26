package server

import (
	"pbs-exporter/internal/metrics"
	"pbs-exporter/internal/pbs"
)

// Server handles the HTTP server and metrics coordination
type Server struct {
	registry *metrics.Registry
	pbsClient *pbs.Client
}

// New creates a new server instance
func New(registry *metrics.Registry, pbsClient *pbs.Client) *Server {
	return &Server{
		registry:  registry,
		pbsClient: pbsClient,
	}
}

// UpdateMetrics updates all metrics by fetching and parsing PBS data
func (s *Server) UpdateMetrics() {
	// Update job metrics
	s.updateJobMetrics()
	
	// Update node metrics
	s.updateNodeMetrics()

	// Update qstat -q summary metrics
	s.updateQueueSummaryMetrics()
}

// updateJobMetrics updates job-related metrics
func (s *Server) updateJobMetrics() {
	// Reset job metrics
	s.registry.ResetJobMetrics()

	// Get qstat output
	output, err := s.pbsClient.GetQstatOutput()
	if err != nil {
		return
	}

	// Parse job data
	jobData := s.pbsClient.ParseQstatOutput(output)

	// Update metrics with parsed data
	s.updateJobMetricsFromData(jobData)
}

// updateNodeMetrics updates node-related metrics
func (s *Server) updateNodeMetrics() {
	// Reset node metrics
	s.registry.ResetNodeMetrics()

	// Get pbsnodes output
	output, err := s.pbsClient.GetPbsnodesOutput()
	if err != nil {
		return
	}

	// Parse node data
	nodeData := s.pbsClient.ParsePbsnodesOutput(output)

	// Update metrics with parsed data
	s.updateNodeMetricsFromData(nodeData)
}

// updateQueueSummaryMetrics updates totals from `qstat -q`
func (s *Server) updateQueueSummaryMetrics() {
	output, err := s.pbsClient.GetQstatQOutput()
	if err != nil {
		return
	}
	running, queued := s.pbsClient.ParseQstatQSummary(output)
	s.registry.QueueSummaryRunning.Set(float64(running))
	s.registry.QueueSummaryQueued.Set(float64(queued))

	// Per-queue values
	runByQ, queByQ := s.pbsClient.ParseQstatQPerQueue(output)
	for q, v := range runByQ {
		s.registry.QueueSummaryRunningByQueue.WithLabelValues(q).Set(float64(v))
	}
	for q, v := range queByQ {
		s.registry.QueueSummaryQueuedByQueue.WithLabelValues(q).Set(float64(v))
		s.registry.QueueQueuedByQueue.WithLabelValues(q).Set(float64(v))
	}
}

// updateJobMetricsFromData updates job metrics from parsed data
func (s *Server) updateJobMetricsFromData(data *pbs.JobData) {
	// Update user job counts
	for user, count := range data.UserJobCount {
		s.registry.RunningJobsByUser.WithLabelValues(user).Set(float64(count))
	}

	// Update queue metrics
	queues := []string{"interactive", "medium", "long", "large", "small", "special", "AISG_debug", "AISG_large", "AISG_guest"}
	for _, queue := range queues {
		s.registry.RunningJobsByQueue.WithLabelValues(queue).Set(float64(data.QueueJobCount[queue]))
		s.registry.JobsInQueue.WithLabelValues(queue).Set(float64(data.QueueTotalCount[queue]))
	}

	// Update status metrics
	for status, count := range data.StatusCount {
		s.registry.JobsByStatus.WithLabelValues(status).Set(float64(count))
	}

	// Update total metrics
	s.registry.TotalRunningJobs.Set(float64(data.TotalRunning))
	s.registry.TotalRJobs.Set(float64(data.TotalR))
	s.registry.TotalHJobs.Set(float64(data.TotalH))
	s.registry.TotalFJobs.Set(float64(data.TotalF))
	s.registry.TotalQJobs.Set(float64(data.TotalQ))
	s.registry.TotalEJobs.Set(float64(data.TotalE))
	s.registry.TotalBJobs.Set(float64(data.TotalB))
	s.registry.TotalAllJobs.Set(float64(data.TotalAll))
}

// updateNodeMetricsFromData updates node metrics from parsed data
func (s *Server) updateNodeMetricsFromData(data *pbs.NodeData) {
	// Update node count metrics
	s.registry.NodeCountFree.Set(float64(data.CountFree))
	s.registry.NodeCountBusy.Set(float64(data.CountBusy))
	s.registry.NodeCountOffline.Set(float64(data.CountOffline))
	s.registry.NodeCountDown.Set(float64(data.CountDown))

	// Update individual node metrics
	for nodeName, nodeInfo := range data.Nodes {
		// Set node state
		var stateValue float64 = 4 // down (default)
		switch nodeInfo.State {
		case "free":
			stateValue = 1
		case "busy":
			stateValue = 2
		case "offline":
			stateValue = 3
		case "down":
			stateValue = 4
		}
		s.registry.NodeState.WithLabelValues(nodeName).Set(stateValue)

		// Set node jobs
		s.registry.NodeJobs.WithLabelValues(nodeName).Set(float64(nodeInfo.Jobs))

		// Set CPU metrics
		usedCpus := nodeInfo.CPUsTotal - nodeInfo.CPUsAvailable
		s.registry.NodeCpusAvailable.WithLabelValues(nodeName).Set(float64(nodeInfo.CPUsAvailable))
		s.registry.NodeCpusUsed.WithLabelValues(nodeName).Set(float64(usedCpus))
		s.registry.NodeCpusTotal.WithLabelValues(nodeName).Set(float64(nodeInfo.CPUsTotal))

		// Set GPU metrics
		usedGpus := nodeInfo.GPUsTotal - nodeInfo.GPUsAvailable
		s.registry.NodeGpusAvailable.WithLabelValues(nodeName).Set(float64(nodeInfo.GPUsAvailable))
		s.registry.NodeGpusUsed.WithLabelValues(nodeName).Set(float64(usedGpus))
		s.registry.NodeGpusTotal.WithLabelValues(nodeName).Set(float64(nodeInfo.GPUsTotal))

		// Set memory metrics
		usedMemory := nodeInfo.MemoryTotal - nodeInfo.MemoryAvailable
		s.registry.NodeMemoryAvailable.WithLabelValues(nodeName).Set(nodeInfo.MemoryAvailable)
		s.registry.NodeMemoryUsed.WithLabelValues(nodeName).Set(usedMemory)
		s.registry.NodeMemoryTotal.WithLabelValues(nodeName).Set(nodeInfo.MemoryTotal)
	}
}

package server

import (
	"time"

	"pbs-exporter/internal/metrics"
	"pbs-exporter/internal/pbs"
)

type Server struct {
	registry  *metrics.Registry
	pbsClient *pbs.Client
}

func New(registry *metrics.Registry, pbsClient *pbs.Client) *Server {
	return &Server{
		registry:  registry,
		pbsClient: pbsClient,
	}
}

// UpdateMetrics refreshes all metrics. Each section runs its PBS command and parses
// the result before touching any metric, so scrapes never see a half-empty set, and
// a failed command keeps the last good values (pbs_exporter_command_success shows it).
func (s *Server) UpdateMetrics() {
	s.updateJobMetrics()
	s.updateNodeMetrics()
	s.updateQueueMetrics()
	s.updateServerMetrics()
}

// timed runs a PBS command and records how long it took
func (s *Server) timed(command string, get func() (string, error)) (string, error) {
	start := time.Now()
	output, err := get()
	s.registry.ExporterCommandDuration.WithLabelValues(command).Set(time.Since(start).Seconds())
	return output, err
}

// record sets the health metrics for a command and reports whether it succeeded
func (s *Server) record(command string, err error) bool {
	if err != nil {
		s.registry.ExporterCommandSuccess.WithLabelValues(command).Set(0)
		return false
	}
	s.registry.ExporterCommandSuccess.WithLabelValues(command).Set(1)
	s.registry.ExporterLastSuccess.WithLabelValues(command).SetToCurrentTime()
	return true
}

func (s *Server) updateJobMetrics() {
	output, err := s.timed("qstat_json", s.pbsClient.GetQstatJobsJSON)
	var jobs []pbs.JobDetail
	var version string
	if err == nil {
		jobs, version, err = s.pbsClient.ParseQstatJobsJSON(output)
	}
	if !s.record("qstat_json", err) {
		return
	}

	s.registry.ResetJobMetrics()
	s.registry.ResetJobDetailMetrics()
	s.registry.PBSVersion.Reset()

	s.registry.PBSVersion.WithLabelValues(version).Set(1)
	s.updateJobMetricsFromData(pbs.BuildJobData(jobs))

	now := time.Now()
	for _, job := range jobs {
		switch job.State {
		case "R":
			for node, gpus := range job.Nodes {
				s.registry.JobInfo.WithLabelValues(job.JobID, job.User, job.Queue, job.Project, job.Name, node).Set(1)
				s.registry.JobGpus.WithLabelValues(job.JobID, node).Set(float64(gpus))
				s.registry.ProjectGpus.WithLabelValues(job.Project, job.State).Add(float64(gpus))
			}
			s.registry.JobWalltimeUsed.WithLabelValues(job.JobID).Set(float64(job.WalltimeUsed))
			s.registry.JobWalltimeRequested.WithLabelValues(job.JobID).Set(float64(job.WalltimeRequested))
		case "Q", "H":
			s.registry.JobQueuedInfo.WithLabelValues(job.JobID, job.User, job.Queue, job.Project, job.Name, job.State).Set(1)
			if !job.QueuedAt.IsZero() {
				s.registry.JobQueuedSeconds.WithLabelValues(job.JobID).Set(now.Sub(job.QueuedAt).Seconds())
			}
			s.registry.ProjectGpus.WithLabelValues(job.Project, job.State).Add(float64(job.GPUsRequested))
		default:
			continue
		}
		s.registry.ProjectJobs.WithLabelValues(job.Project, job.State).Inc()
	}
}

func (s *Server) updateNodeMetrics() {
	output, err := s.timed("pbsnodes", s.pbsClient.GetPbsnodesOutput)
	if !s.record("pbsnodes", err) {
		return
	}

	nodeData := s.pbsClient.ParsePbsnodesOutput(output)
	s.registry.ResetNodeMetrics()
	s.updateNodeMetricsFromData(nodeData)
}

func (s *Server) updateQueueMetrics() {
	output, err := s.timed("qstat_q", s.pbsClient.GetQstatQOutput)
	if !s.record("qstat_q", err) {
		return
	}

	queueData := s.pbsClient.ParseQstatQFull(output)
	s.registry.ResetQueueMetrics()

	running, queued := s.pbsClient.ParseQstatQSummary(output)
	s.registry.QueueSummaryRunning.Set(float64(running))
	s.registry.QueueSummaryQueued.Set(float64(queued))

	for queueName, queueInfo := range queueData.Queues {
		if queueInfo.Enabled {
			s.registry.QueueEnabled.WithLabelValues(queueName).Set(1)
		} else {
			s.registry.QueueEnabled.WithLabelValues(queueName).Set(0)
		}

		if queueInfo.Started {
			s.registry.QueueStarted.WithLabelValues(queueName).Set(1)
		} else {
			s.registry.QueueStarted.WithLabelValues(queueName).Set(0)
		}

		s.registry.QueueRunningJobs.WithLabelValues(queueName).Set(float64(queueInfo.Running))
		s.registry.QueueQueuedJobs.WithLabelValues(queueName).Set(float64(queueInfo.Queued))
		s.registry.QueueQueuedByQueue.WithLabelValues(queueName).Set(float64(queueInfo.Queued))

		if queueInfo.Walltime > 0 {
			s.registry.QueueWalltime.WithLabelValues(queueName).Set(float64(queueInfo.Walltime))
		}
	}
}

func (s *Server) updateServerMetrics() {
	output, err := s.timed("qstat_bf", s.pbsClient.GetQstatBfOutput)
	if !s.record("qstat_bf", err) {
		return
	}

	serverData := s.pbsClient.ParseQstatBf(output)

	if serverData.State == "Active" {
		s.registry.ServerState.Set(1)
	} else {
		s.registry.ServerState.Set(0)
	}

	if serverData.Scheduling {
		s.registry.ServerSchedulingEnabled.Set(1)
	} else {
		s.registry.ServerSchedulingEnabled.Set(0)
	}

	s.registry.ServerTotalJobs.Set(float64(serverData.TotalJobs))
	s.registry.ServerJobsRunning.Set(float64(serverData.JobsRunning))
	s.registry.ServerJobsQueued.Set(float64(serverData.JobsQueued))
	s.registry.ServerJobsHeld.Set(float64(serverData.JobsHeld))
	s.registry.ServerJobsWaiting.Set(float64(serverData.JobsWaiting))
	s.registry.ServerJobsExiting.Set(float64(serverData.JobsExiting))
	s.registry.ServerResourcesNcpus.Set(float64(serverData.ResourcesNcpus))
	s.registry.ServerResourcesMemGb.Set(serverData.ResourcesMemGB)
	s.registry.ServerResourcesNodect.Set(float64(serverData.ResourcesNodect))
	s.registry.ServerLicensesAvailable.Set(float64(serverData.LicensesAvailable))
	s.registry.ServerLicensesUsed.Set(float64(serverData.LicensesUsed))
	s.registry.ServerMaxArraySize.Set(float64(serverData.MaxArraySize))

	if serverData.JobHistoryEnabled {
		s.registry.ServerJobHistoryEnabled.Set(1)
	} else {
		s.registry.ServerJobHistoryEnabled.Set(0)
	}

	s.registry.ServerJobHistoryDuration.Set(float64(serverData.JobHistoryDuration))
}

func (s *Server) updateJobMetricsFromData(data *pbs.JobData) {
	for user, count := range data.UserJobCount {
		s.registry.RunningJobsByUser.WithLabelValues(user).Set(float64(count))
	}

	for user, count := range data.QueuedJobsByUser {
		if count > 0 {
			s.registry.QueuedJobsByUser.WithLabelValues(user).Set(float64(count))
		}
	}

	for queue, count := range data.QueueJobCount {
		s.registry.RunningJobsByQueue.WithLabelValues(queue).Set(float64(count))
	}
	for queue, count := range data.QueueTotalCount {
		s.registry.JobsInQueue.WithLabelValues(queue).Set(float64(count))
	}

	for status, count := range data.StatusCount {
		s.registry.JobsByStatus.WithLabelValues(status).Set(float64(count))
	}

	s.registry.TotalRunningJobs.Set(float64(data.TotalRunning))
	s.registry.TotalRJobs.Set(float64(data.TotalR))
	s.registry.TotalHJobs.Set(float64(data.TotalH))
	s.registry.TotalFJobs.Set(float64(data.TotalF))
	s.registry.TotalQJobs.Set(float64(data.TotalQ))
	s.registry.TotalEJobs.Set(float64(data.TotalE))
	s.registry.TotalBJobs.Set(float64(data.TotalB))
	s.registry.TotalAllJobs.Set(float64(data.TotalAll))
}

func (s *Server) updateNodeMetricsFromData(data *pbs.NodeData) {
	s.registry.NodeCountFree.Set(float64(data.CountFree))
	s.registry.NodeCountBusy.Set(float64(data.CountBusy))
	s.registry.NodeCountOffline.Set(float64(data.CountOffline))
	s.registry.NodeCountDown.Set(float64(data.CountDown))

	var totalCpus, availableCpus, usedCpus int
	var totalGpus, availableGpus, usedGpus int
	var totalMem, availableMem, usedMem float64

	for nodeName, nodeInfo := range data.Nodes {
		var stateValue float64 = 4
		switch nodeInfo.State {
		case "free":
			stateValue = 1
		case "job-busy":
			stateValue = 2
		case "offline":
			stateValue = 3
		case "down":
			stateValue = 4
		}
		s.registry.NodeState.WithLabelValues(nodeName).Set(stateValue)

		s.registry.NodeJobs.WithLabelValues(nodeName).Set(float64(nodeInfo.Jobs))

		usedCpu := nodeInfo.CPUsTotal - nodeInfo.CPUsAvailable
		s.registry.NodeCpusAvailable.WithLabelValues(nodeName).Set(float64(nodeInfo.CPUsAvailable))
		s.registry.NodeCpusUsed.WithLabelValues(nodeName).Set(float64(usedCpu))
		s.registry.NodeCpusTotal.WithLabelValues(nodeName).Set(float64(nodeInfo.CPUsTotal))

		if nodeInfo.CPUsTotal > 0 {
			cpuUtil := float64(usedCpu) / float64(nodeInfo.CPUsTotal) * 100
			s.registry.NodeCpuUtilization.WithLabelValues(nodeName).Set(cpuUtil)
		}

		usedGpu := nodeInfo.GPUsTotal - nodeInfo.GPUsAvailable
		s.registry.NodeGpusAvailable.WithLabelValues(nodeName).Set(float64(nodeInfo.GPUsAvailable))
		s.registry.NodeGpusUsed.WithLabelValues(nodeName).Set(float64(usedGpu))
		s.registry.NodeGpusTotal.WithLabelValues(nodeName).Set(float64(nodeInfo.GPUsTotal))

		if nodeInfo.GPUsTotal > 0 {
			gpuUtil := float64(usedGpu) / float64(nodeInfo.GPUsTotal) * 100
			s.registry.NodeGpuUtilization.WithLabelValues(nodeName).Set(gpuUtil)
		}

		usedMemory := nodeInfo.MemoryTotal - nodeInfo.MemoryAvailable
		s.registry.NodeMemoryAvailable.WithLabelValues(nodeName).Set(nodeInfo.MemoryAvailable)
		s.registry.NodeMemoryUsed.WithLabelValues(nodeName).Set(usedMemory)
		s.registry.NodeMemoryTotal.WithLabelValues(nodeName).Set(nodeInfo.MemoryTotal)

		if nodeInfo.MemoryTotal > 0 {
			memUtil := usedMemory / nodeInfo.MemoryTotal * 100
			s.registry.NodeMemoryUtilization.WithLabelValues(nodeName).Set(memUtil)
		}

		totalCpus += nodeInfo.CPUsTotal
		availableCpus += nodeInfo.CPUsAvailable
		usedCpus += usedCpu
		totalGpus += nodeInfo.GPUsTotal
		availableGpus += nodeInfo.GPUsAvailable
		usedGpus += usedGpu
		totalMem += nodeInfo.MemoryTotal
		availableMem += nodeInfo.MemoryAvailable
		usedMem += usedMemory
	}

	s.registry.ClusterCpusTotal.Set(float64(totalCpus))
	s.registry.ClusterCpusAvailable.Set(float64(availableCpus))
	s.registry.ClusterCpusUsed.Set(float64(usedCpus))
	s.registry.ClusterGpusTotal.Set(float64(totalGpus))
	s.registry.ClusterGpusAvailable.Set(float64(availableGpus))
	s.registry.ClusterGpusUsed.Set(float64(usedGpus))
	s.registry.ClusterMemoryTotalGb.Set(totalMem)
	s.registry.ClusterMemoryAvailableGb.Set(availableMem)
	s.registry.ClusterMemoryUsedGb.Set(usedMem)
}

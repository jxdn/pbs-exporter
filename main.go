package main

import (
        "bufio"
        "log"
        "net/http"
        "os/exec"
        "strconv"
        "strings"
        "time"

        "github.com/prometheus/client_golang/prometheus"
        "github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
        // Running jobs count per user
        runningJobsByUser = prometheus.NewGaugeVec(
                prometheus.GaugeOpts{
                        Name: "qstat_running_jobs_by_user",
                        Help: "Number of running jobs per user",
                },
                []string{"user"},
        )

        // Running jobs count per queue
        runningJobsByQueue = prometheus.NewGaugeVec(
                prometheus.GaugeOpts{
                        Name: "qstat_running_jobs_by_queue",
                        Help: "Number of running jobs per queue",
                },
                []string{"queue"},
        )

        // Jobs count per queue (all statuses)
        jobsInQueue = prometheus.NewGaugeVec(
                prometheus.GaugeOpts{
                        Name: "qstat_jobs_in_queue",
                        Help: "Total number of jobs in each queue",
                },
                []string{"queue"},
        )

        // Total running jobs
        totalRunningJobs = prometheus.NewGauge(
                prometheus.GaugeOpts{
                        Name: "qstat_total_running_jobs",
                        Help: "Total number of running jobs",
                },
        )

        // Total jobs by specific status
        totalRJobs = prometheus.NewGauge(
                prometheus.GaugeOpts{
                        Name: "qstat_total_r_jobs",
                        Help: "Total number of Running (R) jobs",
                },
        )

        totalHJobs = prometheus.NewGauge(
                prometheus.GaugeOpts{
                        Name: "qstat_total_h_jobs",
                        Help: "Total number of Hold (H) jobs",
                },
        )

        totalFJobs = prometheus.NewGauge(
                prometheus.GaugeOpts{
                        Name: "qstat_total_f_jobs",
                        Help: "Total number of Finished (F) jobs",
                },
        )

        totalQJobs = prometheus.NewGauge(
                prometheus.GaugeOpts{
                        Name: "qstat_total_q_jobs",
                        Help: "Total number of Queuing (Q) jobs",
                },
        )

        totalAllJobs = prometheus.NewGauge(
                prometheus.GaugeOpts{
                        Name: "qstat_total_all_jobs",
                        Help: "Total number of all jobs",
                },
        )

        // Jobs by status
        jobsByStatus = prometheus.NewGaugeVec(
                prometheus.GaugeOpts{
                        Name: "qstat_jobs_by_status",
                        Help: "Number of jobs by status",
                },
                []string{"status"},
        )

        // PBS node metrics
        nodeState = prometheus.NewGaugeVec(
                prometheus.GaugeOpts{
                        Name: "pbs_node_state",
                        Help: "Node state (1=free, 2=busy, 3=offline, 4=down)",
                },
                []string{"node"},
        )

        nodeJobs = prometheus.NewGaugeVec(
                prometheus.GaugeOpts{
                        Name: "pbs_node_jobs",
                        Help: "Number of jobs on node",
                },
                []string{"node"},
        )

        nodeCpusAvailable = prometheus.NewGaugeVec(
                prometheus.GaugeOpts{
                        Name: "pbs_node_cpus_available",
                        Help: "Available CPUs on node",
                },
                []string{"node"},
        )

        nodeCpusUsed = prometheus.NewGaugeVec(
                prometheus.GaugeOpts{
                        Name: "pbs_node_cpus_used",
                        Help: "Used CPUs on node",
                },
                []string{"node"},
        )

        nodeCpusTotal = prometheus.NewGaugeVec(
                prometheus.GaugeOpts{
                        Name: "pbs_node_cpus_total",
                        Help: "Total CPUs on node",
                },
                []string{"node"},
        )

        nodeGpusAvailable = prometheus.NewGaugeVec(
                prometheus.GaugeOpts{
                        Name: "pbs_node_gpus_available",
                        Help: "Available GPUs on node",
                },
                []string{"node"},
        )

        nodeGpusUsed = prometheus.NewGaugeVec(
                prometheus.GaugeOpts{
                        Name: "pbs_node_gpus_used",
                        Help: "Used GPUs on node",
                },
                []string{"node"},
        )

        nodeGpusTotal = prometheus.NewGaugeVec(
                prometheus.GaugeOpts{
                        Name: "pbs_node_gpus_total",
                        Help: "Total GPUs on node",
                },
                []string{"node"},
        )

        nodeMemoryAvailable = prometheus.NewGaugeVec(
                prometheus.GaugeOpts{
                        Name: "pbs_node_memory_available_gb",
                        Help: "Available memory on node in GB",
                },
                []string{"node"},
        )

        nodeMemoryUsed = prometheus.NewGaugeVec(
                prometheus.GaugeOpts{
                        Name: "pbs_node_memory_used_gb",
                        Help: "Used memory on node in GB",
                },
                []string{"node"},
        )

        nodeMemoryTotal = prometheus.NewGaugeVec(
                prometheus.GaugeOpts{
                        Name: "pbs_node_memory_total_gb",
                        Help: "Total memory on node in GB",
                },
                []string{"node"},
        )
)

func init() {
        // Don't register with default registry to avoid Go runtime metrics
}

// Convert PBS status codes to descriptive names
func mapStatusToDescription(status string) string {
        switch strings.ToUpper(status) {
        case "F":
                return "Finished"
        case "H":
                return "Hold"
        case "R":
                return "Running"
        case "Q":
                return "Queuing"
        default:
                return status // Return original if unknown
        }
}

// Convert memory string to GB - fix for memory parsing
func parseMemoryToGB(memStr string) float64 {
        memStr = strings.ToLower(strings.TrimSpace(memStr))

        if memStr == "--" || memStr == "" {
                return 0
        }

        var multiplier float64 = 1
        var numStr string

        // Check for units in correct order (longest first)
        if strings.HasSuffix(memStr, "tb") {
                multiplier = 1024
                numStr = strings.TrimSuffix(memStr, "tb")
        } else if strings.HasSuffix(memStr, "gb") {
                multiplier = 1
                numStr = strings.TrimSuffix(memStr, "gb")
        } else if strings.HasSuffix(memStr, "mb") {
                multiplier = 0.001
                numStr = strings.TrimSuffix(memStr, "mb")
        } else if strings.HasSuffix(memStr, "kb") {
                multiplier = 0.000001
                numStr = strings.TrimSuffix(memStr, "kb")
        } else {
                // Assume GB if no unit
                multiplier = 1
                numStr = memStr
        }

        if val, err := strconv.ParseFloat(numStr, 64); err == nil {
                result := val * multiplier
                return result
        }

        return 0
}

// Parse CPU/GPU fraction like "112/112" -> free=112, total=112, used=0
func parseFraction(fracStr string) (free, total int) {
        if fracStr == "--" || fracStr == "" {
                return 0, 0
        }

        parts := strings.Split(fracStr, "/")
        if len(parts) != 2 {
                return 0, 0
        }

        if f, err := strconv.Atoi(parts[0]); err == nil {
                free = f
        }
        if t, err := strconv.Atoi(parts[1]); err == nil {
                total = t
        }

        return free, total
}

func parsePbsnodesOutput(output string) {
        // Reset metrics
        nodeState.Reset()
        nodeJobs.Reset()
        nodeCpusAvailable.Reset()
        nodeCpusUsed.Reset()
        nodeCpusTotal.Reset()
        nodeGpusAvailable.Reset()
        nodeGpusUsed.Reset()
        nodeGpusTotal.Reset()
        nodeMemoryAvailable.Reset()
        nodeMemoryUsed.Reset()
        nodeMemoryTotal.Reset()

        scanner := bufio.NewScanner(strings.NewReader(output))
        lineCount := 0

        for scanner.Scan() {
                line := strings.TrimSpace(scanner.Text())
                lineCount++

                // Skip header lines
                if lineCount <= 2 || line == "" {
                        continue
                }

                // Skip separator line
                if strings.Contains(line, "----") {
                        continue
                }

                // Parse node line
                fields := strings.Fields(line)
                if len(fields) >= 9 {
                        nodeName := fields[0]
                        state := fields[1]
                        njobs := fields[2]
                        memField := fields[5]    // mem f/t
                        cpuField := fields[6]    // ncpus f/t  
                        gpuField := fields[8]    // ngpus f/t

                        // Parse njobs
                        if jobs, err := strconv.Atoi(njobs); err == nil {
                                nodeJobs.WithLabelValues(nodeName).Set(float64(jobs))
                        }

                        // Parse state
                        var stateValue float64 = 4 // down (default)
                        switch state {
                        case "free":
                                stateValue = 1
                        case "busy":
                                stateValue = 2
                        case "offline":
                                stateValue = 3
                        case "down":
                                stateValue = 4
                        }
                        nodeState.WithLabelValues(nodeName).Set(stateValue)

                        // Parse memory (format: "2tb/2tb" means available=2tb, total=2tb, used=total-available)
                        memParts := strings.Split(memField, "/")
                        if len(memParts) == 2 {
                                availableMem := parseMemoryToGB(memParts[0])
                                totalMem := parseMemoryToGB(memParts[1])
                                usedMem := totalMem - availableMem

                                nodeMemoryAvailable.WithLabelValues(nodeName).Set(availableMem)
                                nodeMemoryTotal.WithLabelValues(nodeName).Set(totalMem)
                                nodeMemoryUsed.WithLabelValues(nodeName).Set(usedMem)
                        }

                        // Parse CPUs (format: "112/112" means free=112, total=112)
                        freeCpus, totalCpus := parseFraction(cpuField)
                        usedCpus := totalCpus - freeCpus
                        nodeCpusAvailable.WithLabelValues(nodeName).Set(float64(freeCpus))
                        nodeCpusUsed.WithLabelValues(nodeName).Set(float64(usedCpus))
                        nodeCpusTotal.WithLabelValues(nodeName).Set(float64(totalCpus))

                        // Parse GPUs (format: "8/8" means free=8, total=8)
                        freeGpus, totalGpus := parseFraction(gpuField)
                        usedGpus := totalGpus - freeGpus
                        nodeGpusAvailable.WithLabelValues(nodeName).Set(float64(freeGpus))
                        nodeGpusUsed.WithLabelValues(nodeName).Set(float64(usedGpus))
                        nodeGpusTotal.WithLabelValues(nodeName).Set(float64(totalGpus))
                }
        }
}

func parseQstatOutput(output string) {
        // Reset metrics
        runningJobsByUser.Reset()
        runningJobsByQueue.Reset()
        jobsInQueue.Reset()
        jobsByStatus.Reset()

        // Initialize all queues with 0
        queues := []string{"interactive", "medium", "long", "large", "small", "special", "AISG_debug", "AISG_large", "AISG_guest"}

        userJobCount := make(map[string]int)
        queueJobCount := make(map[string]int)
        queueTotalCount := make(map[string]int)
        statusCount := make(map[string]int)
        
        // Counters for specific status totals
        totalR := 0
        totalH := 0
        totalF := 0
        totalQ := 0
        totalAll := 0
        totalRunning := 0

        // Initialize all queues to 0
        for _, queue := range queues {
                queueJobCount[queue] = 0
                queueTotalCount[queue] = 0
        }

        scanner := bufio.NewScanner(strings.NewReader(output))
        lineCount := 0

        for scanner.Scan() {
                line := strings.TrimSpace(scanner.Text())
                lineCount++

                // Skip header lines
                if lineCount <= 2 || line == "" {
                        continue
                }

                // Skip separator line
                if strings.Contains(line, "----") {
                        continue
                }

                // Parse job line
                fields := strings.Fields(line)
                if len(fields) >= 6 {
                        user := fields[2]
                        status := fields[4]
                        queue := fields[5]

                        // Map status to descriptive name
                        statusDesc := mapStatusToDescription(status)

                        // Count by descriptive status
                        statusCount[statusDesc]++

                        // Count total jobs in each queue
                        queueTotalCount[queue]++

                        // Count totals by original status code
                        totalAll++
                        switch status {
                        case "R":
                                totalR++
                        case "H":
                                totalH++
                        case "F":
                                totalF++
                        case "Q":
                                totalQ++
                        }

                        // Count running jobs by user and queue (check for original "R" status)
                        if status == "R" {
                                userJobCount[user]++
                                queueJobCount[queue]++
                                totalRunning++
                        }
                }
        }

        // Update metrics
        for user, count := range userJobCount {
                runningJobsByUser.WithLabelValues(user).Set(float64(count))
        }

        // Set all queue metrics (including zeros)
        for _, queue := range queues {
                runningJobsByQueue.WithLabelValues(queue).Set(float64(queueJobCount[queue]))
                jobsInQueue.WithLabelValues(queue).Set(float64(queueTotalCount[queue]))
        }

        // Use descriptive status names in metrics
        for status, count := range statusCount {
                jobsByStatus.WithLabelValues(status).Set(float64(count))
        }

        totalRunningJobs.Set(float64(totalRunning))
        
        // Set the new total metrics
        totalRJobs.Set(float64(totalR))
        totalHJobs.Set(float64(totalH))
        totalFJobs.Set(float64(totalF))
        totalQJobs.Set(float64(totalQ))
        totalAllJobs.Set(float64(totalAll))
}

func updateQstatMetrics() {
        cmd := exec.Command("qstat", "-t")
        output, err := cmd.CombinedOutput()

        if err != nil {
                log.Printf("Error running qstat -t: %v", err)
                return
        }

        parseQstatOutput(string(output))
}

func updatePbsnodesMetrics() {
        cmd := exec.Command("pbsnodes", "-aSj")
        output, err := cmd.CombinedOutput()

        if err != nil {
                log.Printf("Error running pbsnodes -aSj: %v", err)
                return
        }

        parsePbsnodesOutput(string(output))
}

func main() {
        // Create custom registry to avoid default Go metrics
        registry := prometheus.NewRegistry()
        registry.MustRegister(runningJobsByUser)
        registry.MustRegister(runningJobsByQueue)
        registry.MustRegister(jobsInQueue)
        registry.MustRegister(totalRunningJobs)
        registry.MustRegister(totalRJobs)
        registry.MustRegister(totalHJobs)
        registry.MustRegister(totalFJobs)
        registry.MustRegister(totalQJobs)
        registry.MustRegister(totalAllJobs)
        registry.MustRegister(jobsByStatus)
        registry.MustRegister(nodeState)
        registry.MustRegister(nodeJobs)
        registry.MustRegister(nodeCpusAvailable)
        registry.MustRegister(nodeCpusUsed)
        registry.MustRegister(nodeCpusTotal)
        registry.MustRegister(nodeGpusAvailable)
        registry.MustRegister(nodeGpusUsed)
        registry.MustRegister(nodeGpusTotal)
        registry.MustRegister(nodeMemoryAvailable)
        registry.MustRegister(nodeMemoryUsed)
        registry.MustRegister(nodeMemoryTotal)

        // Update metrics every 60 seconds
        go func() {
                // Update immediately on start
                updateQstatMetrics()
                updatePbsnodesMetrics()

                ticker := time.NewTicker(60 * time.Second)
                defer ticker.Stop()

                for {
                        <-ticker.C
                        updateQstatMetrics()
                        updatePbsnodesMetrics()
                }
        }()

        // Expose metrics endpoint with custom registry
        http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

        log.Println("PBS cluster monitoring server starting on 0.0.0.0:8888")
        log.Println("Metrics available at http://0.0.0.0:8888/metrics")
        log.Fatal(http.ListenAndServe("0.0.0.0:8888", nil))
}

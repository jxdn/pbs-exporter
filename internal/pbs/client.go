package pbs

import (
	"bufio"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Client handles PBS command execution and data parsing
type Client struct{}

// NewClient creates a new PBS client
func NewClient() *Client {
	return &Client{}
}

// GetQstatOutput executes qstat -t and returns the output
func (c *Client) GetQstatOutput() (string, error) {
	cmd := exec.Command("qstat", "-t")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error running qstat -t: %v", err)
		return "", err
	}
	return string(output), nil
}

// GetPbsnodesOutput executes pbsnodes -aSj and returns the output
func (c *Client) GetPbsnodesOutput() (string, error) {
	cmd := exec.Command("pbsnodes", "-aSj")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error running pbsnodes -aSj: %v", err)
		return "", err
	}
	return string(output), nil
}

// GetQstatQOutput executes qstat -q and returns the output
func (c *Client) GetQstatQOutput() (string, error) {
	cmd := exec.Command("qstat", "-q")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error running qstat -q: %v", err)
		return "", err
	}
	return string(output), nil
}

// GetQstatBfOutput executes qstat -Bf and returns the output
func (c *Client) GetQstatBfOutput() (string, error) {
	cmd := exec.Command("qstat", "-Bf")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error running qstat -Bf: %v", err)
		return "", err
	}
	return string(output), nil
}

func (c *Client) GetPbsVersion() string {
	cmd := exec.Command("qstat", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		cmd = exec.Command("pbsnodes", "--version")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return "unknown"
		}
	}
	re := regexp.MustCompile(`\d{4}\.\d+\.\d+`)
	version := re.FindString(string(output))
	if version != "" {
		return version
	}
	return "unknown"
}

// JobData represents parsed job information
type JobData struct {
	UserJobCount     map[string]int
	QueuedJobsByUser map[string]int
	QueueJobCount    map[string]int
	QueueTotalCount  map[string]int
	StatusCount      map[string]int
	TotalR           int
	TotalH           int
	TotalF           int
	TotalQ           int
	TotalE           int
	TotalB           int
	TotalAll         int
	TotalRunning     int
}

// NodeData represents parsed node information
type NodeData struct {
	Nodes        map[string]NodeInfo
	CountFree    int
	CountBusy    int
	CountOffline int
	CountDown    int
}

// NodeInfo represents information about a single node
type NodeInfo struct {
	State           string
	Jobs            int
	CPUsAvailable   int
	CPUsTotal       int
	GPUsAvailable   int
	GPUsTotal       int
	MemoryAvailable float64
	MemoryTotal     float64
}

// QueueInfo represents information about a single queue
type QueueInfo struct {
	Running  int
	Queued   int
	Enabled  bool
	Started  bool
	Walltime int
}

// QueueData represents parsed queue information from qstat -q
type QueueData struct {
	Queues map[string]QueueInfo
}

// ServerData represents parsed server information from qstat -Bf
type ServerData struct {
	State              string
	Scheduling         bool
	TotalJobs          int
	JobsRunning        int
	JobsQueued         int
	JobsHeld           int
	JobsWaiting        int
	JobsExiting        int
	ResourcesNcpus     int
	ResourcesMemGB     float64
	ResourcesNodect    int
	LicensesAvailable  int
	LicensesUsed       int
	MaxArraySize       int
	JobHistoryEnabled  bool
	JobHistoryDuration int
}

// ParseQstatOutput parses qstat output and returns structured job data
func (c *Client) ParseQstatOutput(output string) *JobData {
	data := &JobData{
		UserJobCount:     make(map[string]int),
		QueuedJobsByUser: make(map[string]int),
		QueueJobCount:    make(map[string]int),
		QueueTotalCount:  make(map[string]int),
		StatusCount:      make(map[string]int),
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
			data.StatusCount[statusDesc]++

			// Count total jobs in each queue
			data.QueueTotalCount[queue]++

			// Count totals by original status code
			data.TotalAll++
			switch status {
			case "R":
				data.TotalR++
			case "H":
				data.TotalH++
			case "F":
				data.TotalF++
			case "Q":
				data.TotalQ++
			case "E":
				data.TotalE++
			case "B":
				data.TotalB++
			}

			// Count running jobs by user and queue (check for original "R" status)
			if status == "R" {
				data.UserJobCount[user]++
				data.QueueJobCount[queue]++
				data.TotalRunning++
			}

			// Count queued jobs by user (check for original "Q" status)
			if status == "Q" {
				data.QueuedJobsByUser[user]++
			}
		}
	}

	return data
}

// ParseQstatQSummary parses `qstat -q` output and returns totals for running and queued jobs
func (c *Client) ParseQstatQSummary(output string) (totalRunning int, totalQueued int) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	seenSeparator := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Detect separator
		if strings.HasPrefix(line, "---") {
			seenSeparator = true
			continue
		}
		if seenSeparator {
			// Totals line typically: "55     2"
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if v, err := strconv.Atoi(fields[0]); err == nil {
					totalRunning = v
				}
				if v, err := strconv.Atoi(fields[1]); err == nil {
					totalQueued = v
				}
				return
			}
		}

		// Fallback: sum per-queue lines by extracting numeric fields before state
		fields := strings.Fields(line)
		if len(fields) < 3 || strings.EqualFold(fields[0], "Queue") || strings.EqualFold(fields[0], "server:") {
			continue
		}
		// Collect ints in this line
		var nums []int
		for _, f := range fields {
			if n, err := strconv.Atoi(f); err == nil {
				nums = append(nums, n)
			}
		}
		if len(nums) >= 2 {
			// Heuristic: last two numbers are Run and Que
			totalRunning += nums[len(nums)-2]
			totalQueued += nums[len(nums)-1]
		}
	}
	return
}

// ParseQstatQPerQueue parses `qstat -q` output and returns per-queue running and queued counts
func (c *Client) ParseQstatQPerQueue(output string) (runningByQueue map[string]int, queuedByQueue map[string]int) {
	runningByQueue = make(map[string]int)
	queuedByQueue = make(map[string]int)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "server:") || strings.HasPrefix(strings.ToLower(line), "queue ") || strings.HasPrefix(line, "---") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		queueName := fields[0]
		// Extract integers in line
		var nums []int
		for _, f := range fields {
			if n, err := strconv.Atoi(f); err == nil {
				nums = append(nums, n)
			}
		}
		if len(nums) >= 2 {
			// Heuristic: last two numbers are Run and Que
			runningByQueue[queueName] = nums[len(nums)-2]
			queuedByQueue[queueName] = nums[len(nums)-1]
		}
	}
	return
}

// ParseQstatQFull parses `qstat -q` output and returns full queue data including state
func (c *Client) ParseQstatQFull(output string) *QueueData {
	data := &QueueData{
		Queues: make(map[string]QueueInfo),
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "server:") || strings.HasPrefix(strings.ToLower(line), "queue ") || strings.HasPrefix(line, "---") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		queueName := fields[0]
		walltimeStr := fields[2]
		stateStr := ""
		if len(fields) >= 10 {
			stateStr = fields[len(fields)-2] + " " + fields[len(fields)-1]
		} else if len(fields) >= 9 {
			stateStr = fields[len(fields)-1]
		}

		var nums []int
		for _, f := range fields {
			if n, err := strconv.Atoi(f); err == nil {
				nums = append(nums, n)
			}
		}

		running := 0
		queued := 0
		if len(nums) >= 2 {
			running = nums[len(nums)-2]
			queued = nums[len(nums)-1]
		}

		enabled := strings.Contains(stateStr, "E")
		started := strings.Contains(stateStr, "R")

		walltime := parseWalltimeToSeconds(walltimeStr)

		data.Queues[queueName] = QueueInfo{
			Running:  running,
			Queued:   queued,
			Enabled:  enabled,
			Started:  started,
			Walltime: walltime,
		}
	}
	return data
}

// ParseQstatBf parses `qstat -Bf` output and returns server data
func (c *Client) ParseQstatBf(output string) *ServerData {
	data := &ServerData{}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "server_state") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				data.State = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(line, "scheduling") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				data.Scheduling = strings.TrimSpace(parts[1]) == "True"
			}
		} else if strings.HasPrefix(line, "total_jobs") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				data.TotalJobs, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
			}
		} else if strings.HasPrefix(line, "state_count") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				parseStateCount(strings.TrimSpace(parts[1]), data)
			}
		} else if strings.HasPrefix(line, "resources_assigned.ncpus") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				data.ResourcesNcpus, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
			}
		} else if strings.HasPrefix(line, "resources_assigned.mem") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				data.ResourcesMemGB = parseMemoryToGB(strings.TrimSpace(parts[1]))
			}
		} else if strings.HasPrefix(line, "resources_assigned.nodect") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				data.ResourcesNodect, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
			}
		} else if strings.HasPrefix(line, "license_count") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				parseLicenseCount(strings.TrimSpace(parts[1]), data)
			}
		} else if strings.HasPrefix(line, "max_array_size") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				data.MaxArraySize, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
			}
		} else if strings.HasPrefix(line, "job_history_enable") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				data.JobHistoryEnabled = strings.TrimSpace(parts[1]) == "True"
			}
		} else if strings.HasPrefix(line, "job_history_duration") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				data.JobHistoryDuration = parseWalltimeToHours(strings.TrimSpace(parts[1]))
			}
		}
	}

	return data
}

func parseStateCount(s string, data *ServerData) {
	parts := strings.Fields(s)
	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val, _ := strconv.Atoi(strings.TrimSpace(kv[1]))
		switch key {
		case "Running":
			data.JobsRunning = val
		case "Queued":
			data.JobsQueued = val
		case "Held":
			data.JobsHeld = val
		case "Waiting":
			data.JobsWaiting = val
		case "Exiting":
			data.JobsExiting = val
		}
	}
}

func parseLicenseCount(s string, data *ServerData) {
	parts := strings.Fields(s)
	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val, _ := strconv.Atoi(strings.TrimSpace(kv[1]))
		switch key {
		case "Avail_Global":
			data.LicensesAvailable = val
		case "Used":
			data.LicensesUsed = val
		}
	}
}

func parseWalltimeToSeconds(s string) int {
	if s == "--" || s == "" {
		return 0
	}

	parts := strings.Split(s, ":")
	if len(parts) == 3 {
		hours, _ := strconv.Atoi(parts[0])
		mins, _ := strconv.Atoi(parts[1])
		secs, _ := strconv.Atoi(parts[2])
		return hours*3600 + mins*60 + secs
	} else if len(parts) == 2 {
		mins, _ := strconv.Atoi(parts[0])
		secs, _ := strconv.Atoi(parts[1])
		return mins*60 + secs
	}
	return 0
}

func parseWalltimeToHours(s string) int {
	if s == "--" || s == "" {
		return 0
	}

	parts := strings.Split(s, ":")
	if len(parts) >= 1 {
		hours, _ := strconv.Atoi(parts[0])
		return hours
	}
	return 0
}

// ParsePbsnodesOutput parses pbsnodes output and returns structured node data
func (c *Client) ParsePbsnodesOutput(output string) *NodeData {
	data := &NodeData{
		Nodes: make(map[string]NodeInfo),
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

		// Parse node line
		fields := strings.Fields(line)
		if len(fields) >= 9 {
			nodeName := fields[0]
			state := fields[1]
			njobs := fields[2]
			memField := fields[5] // mem f/t
			cpuField := fields[6] // ncpus f/t
			gpuField := fields[8] // ngpus f/t

			// Parse njobs
			jobs := 0
			if j, err := strconv.Atoi(njobs); err == nil {
				jobs = j
			}

			// Skip nodes with state-unknown (node not reachable)
			if state == "state-unknown" {
				continue
			}

			// Parse state and count
			// Normalize state: <various> -> job-busy
			normalizedState := state
			if state == "<various>" {
				normalizedState = "job-busy"
			}

			switch normalizedState {
			case "free":
				data.CountFree++
			case "job-busy":
				data.CountBusy++
			case "offline":
				data.CountOffline++
			case "down":
				data.CountDown++
			default:
				data.CountDown++
			}

			// Store normalized state in NodeInfo
			state = normalizedState

			// Parse memory
			memParts := strings.Split(memField, "/")
			availableMem := float64(0)
			totalMem := float64(0)
			if len(memParts) == 2 {
				availableMem = parseMemoryToGB(memParts[0])
				totalMem = parseMemoryToGB(memParts[1])
			}

			// Parse CPUs
			freeCpus, totalCpus := parseFraction(cpuField)

			// Parse GPUs
			freeGpus, totalGpus := parseFraction(gpuField)

			data.Nodes[nodeName] = NodeInfo{
				State:           normalizedState,
				Jobs:            jobs,
				CPUsAvailable:   freeCpus,
				CPUsTotal:       totalCpus,
				GPUsAvailable:   freeGpus,
				GPUsTotal:       totalGpus,
				MemoryAvailable: availableMem,
				MemoryTotal:     totalMem,
			}
		}
	}

	return data
}

// Helper functions

// mapStatusToDescription converts PBS status codes to descriptive names
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
	case "E":
		return "Error"
	case "B":
		return "ArrayJobRunning"
	default:
		return status // Return original if unknown
	}
}

// parseMemoryToGB converts memory string to GB
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

// parseFraction parses CPU/GPU fraction like "112/112" -> free=112, total=112
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

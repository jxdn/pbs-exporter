package pbs

import (
	"bufio"
	"log"
	"os/exec"
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

// JobData represents parsed job information
type JobData struct {
	UserJobCount   map[string]int
	QueueJobCount  map[string]int
	QueueTotalCount map[string]int
	StatusCount    map[string]int
	TotalR         int
	TotalH         int
	TotalF         int
	TotalQ         int
	TotalE         int
	TotalB         int
	TotalAll       int
	TotalRunning   int
}

// NodeData represents parsed node information
type NodeData struct {
	Nodes          map[string]NodeInfo
	CountFree      int
	CountBusy      int
	CountOffline   int
	CountDown      int
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

// ParseQstatOutput parses qstat output and returns structured job data
func (c *Client) ParseQstatOutput(output string) *JobData {
	data := &JobData{
		UserJobCount:   make(map[string]int),
		QueueJobCount:  make(map[string]int),
		QueueTotalCount: make(map[string]int),
		StatusCount:    make(map[string]int),
	}

	// Initialize all queues with 0
	queues := []string{"interactive", "medium", "long", "large", "small", "special", "AISG_debug", "AISG_large", "AISG_guest"}
	for _, queue := range queues {
		data.QueueJobCount[queue] = 0
		data.QueueTotalCount[queue] = 0
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
		}
	}

	return data
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
			memField := fields[5]    // mem f/t
			cpuField := fields[6]    // ncpus f/t  
			gpuField := fields[8]    // ngpus f/t

			// Parse njobs
			jobs := 0
			if j, err := strconv.Atoi(njobs); err == nil {
				jobs = j
			}

			// Parse state and count
			switch state {
			case "free":
				data.CountFree++
			case "busy":
				data.CountBusy++
			case "offline":
				data.CountOffline++
			case "down":
				data.CountDown++
			default:
				data.CountDown++ // unknown states counted as down
			}

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
				State:           state,
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

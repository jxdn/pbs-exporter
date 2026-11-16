package pbs

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Client handles PBS command execution and data parsing
type Client struct {
	// Timeout bounds each PBS command so a stalled PBS server cannot block updates forever
	Timeout time.Duration
}

// NewClient creates a new PBS client
func NewClient() *Client {
	return &Client{Timeout: 45 * time.Second}
}

var versionRe = regexp.MustCompile(`\d{4}\.\d+\.\d+`)

// run executes a PBS command with c.Timeout and returns its stdout
func (c *Client) run(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stderr = &stderr
	// On timeout kill the whole process group (children too), and stop waiting for
	// output pipes a leftover child might still hold open
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	output, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		err = fmt.Errorf("timed out after %s", c.Timeout)
	}
	if err != nil {
		log.Printf("Error running %s %s: %v %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
		return "", err
	}
	return string(output), nil
}

// GetPbsnodesOutput executes pbsnodes -aSj and returns the output
func (c *Client) GetPbsnodesOutput() (string, error) {
	return c.run("pbsnodes", "-aSj")
}

// GetQstatQueuesJSON executes qstat -Qf -F json and returns the output
func (c *Client) GetQstatQueuesJSON() (string, error) {
	return c.run("qstat", "-Qf", "-F", "json")
}

// GetQstatBfOutput executes qstat -Bf and returns the output
func (c *Client) GetQstatBfOutput() (string, error) {
	return c.run("qstat", "-Bf")
}

// GetQstatJobsJSON executes qstat -f -F json -t and returns the output
func (c *Client) GetQstatJobsJSON() (string, error) {
	return c.run("qstat", "-f", "-F", "json", "-t")
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

// JobDetail represents a single job from qstat -f -F json
type JobDetail struct {
	JobID             string
	State             string // PBS job state: R, Q, H, F, E, B, ...
	User              string
	Queue             string
	Project           string
	Name              string
	Nodes             map[string]int // node name -> GPUs allocated on that node (running jobs only)
	NodeCPUs          map[string]int // node name -> CPUs allocated on that node (running jobs only)
	GPUsRequested     int
	CPUsRequested     int     // Resource_List.ncpus
	MemRequested      float64 // Resource_List.mem in bytes, 0 if unset
	WalltimeUsed      int     // seconds
	WalltimeRequested int     // seconds
	QueuedAt          time.Time
	Comment           string     // scheduler comment, e.g. "Not Running: Insufficient amount of resource: ngpus"
	HoldTypes         string     // Hold_Types: n (none), u (user), o (operator), s (system), p (password)
	Budget            *JobBudget // from Resource_List.am_job_cache (Budgets hook); nil if absent or unreadable
}

// JobBudget is what the PBS Budgets (Allocation Manager) hook charges a job for,
// read from Resource_List.am_job_cache, e.g.
// {"job.walltime": 3600, "job.ncpus": 12.0, "job.ngpus": 1.0, "job.queue_charge_rate": 1.0}
type JobBudget struct {
	Walltime   int     // seconds the job is charged for (requested walltime)
	NCPUs      float64 // CPUs charged
	NGPUs      float64 // GPUs charged
	ChargeRate float64 // queue_charge_rate: 1 = normal, 0 = free queue
}

// GPUHoursRequested is GPUs x requested hours x charge rate: what the job is
// charged in full if it runs its whole walltime
func (b *JobBudget) GPUHoursRequested() float64 {
	return b.NGPUs * float64(b.Walltime) / 3600 * b.ChargeRate
}

// GPUHoursUsed is GPUs x hours used so far x charge rate
func (b *JobBudget) GPUHoursUsed(walltimeUsed int) float64 {
	return b.NGPUs * float64(walltimeUsed) / 3600 * b.ChargeRate
}

// QueueInfo represents a single queue from qstat -Qf -F json
type QueueInfo struct {
	Type      string // Execution or Route
	Enabled   bool
	Started   bool
	Walltime  int // resources_max.walltime in seconds, 0 if unset
	TotalJobs int
	Jobs      map[string]int // jobs per state from state_count: Queued, Held, Running, ...
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

// BuildJobData counts jobs by user, queue and state (the qstat_* job metrics)
func BuildJobData(jobs []JobDetail) *JobData {
	data := &JobData{
		UserJobCount:     make(map[string]int),
		QueuedJobsByUser: make(map[string]int),
		QueueJobCount:    make(map[string]int),
		QueueTotalCount:  make(map[string]int),
		StatusCount:      make(map[string]int),
	}

	for _, job := range jobs {
		data.StatusCount[mapStatusToDescription(job.State)]++
		data.QueueTotalCount[job.Queue]++

		data.TotalAll++
		switch job.State {
		case "R":
			data.TotalR++
			data.UserJobCount[job.User]++
			data.QueueJobCount[job.Queue]++
			data.TotalRunning++
		case "H":
			data.TotalH++
		case "F":
			data.TotalF++
		case "Q":
			data.TotalQ++
			data.QueuedJobsByUser[job.User]++
		case "E":
			data.TotalE++
		case "B":
			data.TotalB++
		}
	}

	return data
}

// ParseQstatJobsJSON parses `qstat -f -F json` output and returns every job
// (any state) plus the PBS version (e.g. 2024.1.0, or "unknown")
func (c *Client) ParseQstatJobsJSON(output string) ([]JobDetail, string, error) {
	var parsed struct {
		PbsVersion string `json:"pbs_version"`
		Jobs       map[string]struct {
			JobName       string          `json:"Job_Name"`
			JobOwner      string          `json:"Job_Owner"`
			JobState      string          `json:"job_state"`
			Queue         string          `json:"queue"`
			Project       string          `json:"project"`
			ExecVnode     string          `json:"exec_vnode"`
			Comment       string          `json:"comment"`
			HoldTypes     string          `json:"Hold_Types"`
			Qtime         json.RawMessage `json:"qtime"`
			ResourcesUsed struct {
				Walltime string `json:"walltime"`
			} `json:"resources_used"`
			ResourceList struct {
				Walltime   string          `json:"walltime"`
				NGPUs      json.RawMessage `json:"ngpus"`
				NCPUs      json.RawMessage `json:"ncpus"`
				Mem        string          `json:"mem"`
				AmJobCache json.RawMessage `json:"am_job_cache"`
			} `json:"Resource_List"`
		} `json:"Jobs"`
	}
	output = sanitizeAmJobCache(output)

	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		log.Printf("Error parsing qstat JSON output: %v", err)
		return nil, "", err
	}

	version := versionRe.FindString(parsed.PbsVersion)
	if version == "" {
		version = "unknown"
	}

	jobs := make([]JobDetail, 0, len(parsed.Jobs))
	for jobID, j := range parsed.Jobs {
		user, _, _ := strings.Cut(j.JobOwner, "@")
		project := j.Project
		if project == "" {
			project = "none"
		}
		job := JobDetail{
			JobID:             jobID,
			State:             j.JobState,
			User:              user,
			Queue:             j.Queue,
			Project:           project,
			Name:              j.JobName,
			GPUsRequested:     int(rawNumber(j.ResourceList.NGPUs)),
			CPUsRequested:     int(rawNumber(j.ResourceList.NCPUs)),
			MemRequested:      parseMemoryToBytes(j.ResourceList.Mem),
			WalltimeUsed:      parseWalltimeToSeconds(j.ResourcesUsed.Walltime),
			WalltimeRequested: parseWalltimeToSeconds(j.ResourceList.Walltime),
			QueuedAt:          parsePbsTime(j.Qtime),
			Comment:           j.Comment,
			HoldTypes:         j.HoldTypes,
			Budget:            parseAmJobCache(j.ResourceList.AmJobCache),
		}
		if j.JobState == "R" {
			job.Nodes = parseExecVnodeGPUs(j.ExecVnode)
			job.NodeCPUs = parseExecVnodeResource(j.ExecVnode, "ncpus")
			// exec_vnode without ncpus on a single node: fall back to Resource_List.ncpus
			if len(job.NodeCPUs) == 1 && job.CPUsRequested > 0 {
				for node, n := range job.NodeCPUs {
					if n == 0 {
						job.NodeCPUs[node] = job.CPUsRequested
					}
				}
			}
		}
		jobs = append(jobs, job)
	}
	return jobs, version, nil
}

// sanitizeAmJobCache replaces am_job_cache values that are not valid JSON with
// null. Some PBS versions write this field with broken quoting (for example
// `""{}\"`), which would otherwise make the whole qstat output unparseable.
// Valid values are kept so the budget fields can be read.
func sanitizeAmJobCache(output string) string {
	const key = `"am_job_cache":`
	if !strings.Contains(output, key) {
		return output
	}
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, key) {
			continue
		}
		comma := strings.HasSuffix(trimmed, ",")
		value := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, key), ","))
		if json.Valid([]byte(value)) {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		lines[i] = indent + key + "null"
		if comma {
			lines[i] += ","
		}
	}
	return strings.Join(lines, "\n")
}

// parseAmJobCache reads am_job_cache, a JSON object stored as a JSON string.
// It returns nil when the field is absent, null or not readable.
func parseAmJobCache(raw json.RawMessage) *JobBudget {
	var text string
	if len(raw) == 0 || json.Unmarshal(raw, &text) != nil || text == "" {
		return nil
	}
	var cache map[string]json.RawMessage
	if json.Unmarshal([]byte(text), &cache) != nil {
		return nil
	}
	rate, hasRate := cache["job.queue_charge_rate"]
	if !hasRate {
		return nil
	}
	return &JobBudget{
		Walltime:   int(rawNumber(cache["job.walltime"])),
		NCPUs:      rawNumber(cache["job.ncpus"]),
		NGPUs:      rawNumber(cache["job.ngpus"]),
		ChargeRate: rawNumber(rate),
	}
}

// rawString returns a JSON value as text, whether it was a JSON string or number
func rawString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return strings.TrimSpace(string(raw))
}

// rawNumber returns a JSON string or number as float64 (0 if absent or not numeric)
func rawNumber(raw json.RawMessage) float64 {
	n, _ := strconv.ParseFloat(rawString(raw), 64)
	return n
}

// parsePbsTime parses a PBS timestamp, either "Mon Oct 19 09:16:22 2026" (server
// local time) or epoch seconds; returns the zero time if empty or unparseable
func parsePbsTime(raw json.RawMessage) time.Time {
	s := rawString(raw)
	if epoch, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(epoch, 0)
	}
	t, err := time.ParseInLocation("Mon Jan _2 15:04:05 2006", s, time.Local)
	if err != nil {
		return time.Time{}
	}
	return t
}

// parseExecVnodeGPUs parses exec_vnode like "(hopper-04:ngpus=1:ncpus=12)+(hopper-05:ncpus=4)"
// and returns GPUs per node (nodes without ngpus map to 0)
func parseExecVnodeGPUs(execVnode string) map[string]int {
	return parseExecVnodeResource(execVnode, "ngpus")
}

// parseExecVnodeResource sums an integer resource (e.g. ngpus, ncpus) per node
// from exec_vnode; nodes without that resource map to 0
func parseExecVnodeResource(execVnode, resource string) map[string]int {
	prefix := resource + "="
	nodes := make(map[string]int)
	for _, chunk := range strings.Split(execVnode, "+") {
		chunk = strings.Trim(strings.TrimSpace(chunk), "()")
		if chunk == "" {
			continue
		}
		parts := strings.Split(chunk, ":")
		node := parts[0]
		nodes[node] += 0
		for _, kv := range parts[1:] {
			if v, ok := strings.CutPrefix(kv, prefix); ok {
				if n, err := strconv.Atoi(v); err == nil {
					nodes[node] += n
				}
			}
		}
	}
	return nodes
}

// ParseQstatQueuesJSON parses `qstat -Qf -F json` output into queues by name.
// Every queue PBS reports is returned, so new queues appear without code changes.
func (c *Client) ParseQstatQueuesJSON(output string) (map[string]QueueInfo, error) {
	var parsed struct {
		Queue map[string]struct {
			QueueType    string          `json:"queue_type"`
			Enabled      json.RawMessage `json:"enabled"`
			Started      json.RawMessage `json:"started"`
			TotalJobs    json.RawMessage `json:"total_jobs"`
			StateCount   string          `json:"state_count"`
			ResourcesMax struct {
				Walltime json.RawMessage `json:"walltime"`
			} `json:"resources_max"`
		} `json:"Queue"`
	}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		log.Printf("Error parsing qstat -Qf JSON output: %v", err)
		return nil, err
	}

	queues := make(map[string]QueueInfo, len(parsed.Queue))
	for name, q := range parsed.Queue {
		info := QueueInfo{
			Type:      q.QueueType,
			Enabled:   strings.EqualFold(rawString(q.Enabled), "true"),
			Started:   strings.EqualFold(rawString(q.Started), "true"),
			Walltime:  parseWalltimeToSeconds(rawString(q.ResourcesMax.Walltime)),
			TotalJobs: int(rawNumber(q.TotalJobs)),
			Jobs:      make(map[string]int),
		}
		// state_count looks like "Transit:0 Queued:5 Held:1 Waiting:0 Running:32 Exiting:0 Begun:5"
		for _, part := range strings.Fields(q.StateCount) {
			if state, n, ok := strings.Cut(part, ":"); ok {
				if v, err := strconv.Atoi(n); err == nil {
					info.Jobs[state] = v
				}
			}
		}
		queues[name] = info
	}
	return queues, nil
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

// parseMemoryToBytes converts a PBS size such as "450gb", "471859200kb" or
// "100GB" to bytes. PBS units are powers of 1024; a bare number is bytes and
// "w" means words (8 bytes). Returns 0 if empty or unparseable.
func parseMemoryToBytes(s string) float64 {
	s = strings.ToLower(strings.TrimSpace(s))
	units := []struct {
		suffix string
		mult   float64
	}{
		{"pb", 1 << 50}, {"tb", 1 << 40}, {"gb", 1 << 30}, {"mb", 1 << 20}, {"kb", 1 << 10},
		{"pw", 8 << 50}, {"tw", 8 << 40}, {"gw", 8 << 30}, {"mw", 8 << 20}, {"kw", 8 << 10},
		{"b", 1}, {"w", 8},
	}
	mult := 1.0
	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			s, mult = strings.TrimSuffix(s, u.suffix), u.mult
			break
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return 0
	}
	return v * mult
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

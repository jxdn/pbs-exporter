package pbs

import (
	"os"
	"sort"
	"testing"
)

func TestWaitReason(t *testing.T) {
	cases := []struct {
		state, comment, hold string
		reason, detail       string
	}{
		// comments seen on Hopper (PBS 2024.1)
		{"Q", "Not Running: User has reached queue small running job limit.", "n", ReasonUserLimit, "queue small running job"},
		{"Q", "Not Running: User has reached queue interactive running job limit.", "n", ReasonUserLimit, "queue interactive running job"},
		{"Q", "Not Running: Insufficient amount of resource: ngpus ", "n", ReasonResource, "ngpus"},
		{"Q", "Not Running: Insufficient amount of resource: ngpus (R: 1 A: 0 T: 168)", "n", ReasonResource, "ngpus"},
		{"Q", "Not Running: Insufficient amount of resource: ncpus ", "n", ReasonResource, "ncpus"},
		{"Q", "Can Never Run: Insufficient amount of resource: host (hopper-01 != hopper-02,hopper-03)", "n", ReasonCanNeverRun, "host"},
		{"Q", "Can Never Run: Insufficient amount of resource: gpu_model (auto != H100,H200)", "n", ReasonCanNeverRun, "gpu_model"},
		{"Q", "Not Running: PBS Error: Budgets-am_hook: Project has already ended.", "n", ReasonBudget, "Project has already ended"},
		{"Q", "Not Running: PBS Error: Budgets-am_hook: Required budget is not available for this transaction.", "n", ReasonBudget, "Required budget is not available for this transaction"},
		{"Q", "Job Array Began at Fri Nov 13 at 10:02", "n", ReasonArraySubjob, ""},
		{"Q", "Job Array Held, too many failed attempts to run subjob 628953[3].hopper-m-02", "n", ReasonHeldFailed, ""},
		{"Q", "", "n", ReasonNotEvaluated, ""},
		{"H", "job held, too many failed attempts to run", "s", ReasonHeldFailed, ""},
		{"H", "Job held by e0555783 on Wed Nov 11 10:00:00 2026", "u", ReasonHeldUser, "u"},
		{"H", "Job held by e1373662 on Mon Oct 19 10:00:00 2026", "su", ReasonHeldSystem, "su"},
		{"H", "Job run at Wed Oct 21 at 00:36 on (hopper-04:ngpus=1:ncpus=12:mem=100kb)", "s", ReasonHeldSystem, "s"},
		{"H", "", "o", ReasonHeldOperator, "o"},
		{"W", "", "n", ReasonStartTime, ""},
		// standard PBS scheduler messages
		{"Q", "Not Running: User has reached server running job limit.", "n", ReasonUserLimit, "server running job"},
		{"Q", "Not Running: Group has reached queue gpu running limit.", "n", ReasonGroupLimit, "queue gpu running"},
		{"Q", "Not Running: Project has reached server running limit.", "n", ReasonProjectLimit, "server running"},
		{"Q", "Not Running: would exceed user bob's limit on resource ngpus in queue gpu", "n", ReasonUserLimit, "queue gpu ngpus"},
		{"Q", "Not Running: would exceed per-user limit on resource ncpus in complex", "n", ReasonUserLimit, "server ncpus"},
		{"Q", "Not Running: would exceed overall limit on resource ngpus in queue gpu", "n", ReasonQueueLimit, "queue gpu ngpus"},
		{"Q", "Not Running: Queue small job limit has been reached.", "n", ReasonQueueLimit, "queue small"},
		{"Q", "Not Running: Job would conflict with reservation or top job", "n", ReasonTopJob, ""},
		{"Q", "Not Running: Draining system to allow top job to run", "n", ReasonTopJob, ""},
		{"Q", "Not Running: Not enough free nodes available", "n", ReasonResource, "nodes"},
		{"Q", "Not Running: Something new 123 happened.", "n", ReasonOther, "Something new N happened"},
	}
	for _, c := range cases {
		reason, detail := WaitReason(c.state, c.comment, c.hold)
		if reason != c.reason || detail != c.detail {
			t.Errorf("WaitReason(%q, %q, %q) = %q, %q; want %q, %q", c.state, c.comment, c.hold, reason, detail, c.reason, c.detail)
		}
	}
}

// TestWaitReasonRealData classifies a real `qstat -f -F json -t` dump when
// PBS_QSTAT_JSON points to one, and prints the reason counts (go test -v)
func TestWaitReasonRealData(t *testing.T) {
	path := os.Getenv("PBS_QSTAT_JSON")
	if path == "" {
		t.Skip("set PBS_QSTAT_JSON to a qstat -f -F json -t dump")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	jobs, _, err := NewClient().ParseQstatJobsJSON(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	examples := map[string]string{}
	for _, j := range jobs {
		if j.State != "Q" && j.State != "H" && j.State != "W" {
			continue
		}
		reason, detail := WaitReason(j.State, j.Comment, j.HoldTypes)
		key := reason + " | " + detail
		counts[key]++
		if _, ok := examples[key]; !ok {
			examples[key] = j.Comment
		}
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(a, b int) bool { return counts[keys[a]] > counts[keys[b]] })
	for _, k := range keys {
		t.Logf("%5d  %-55s  e.g. %q", counts[k], k, examples[k])
	}
}

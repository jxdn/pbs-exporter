package pbs

import (
	"os"
	"reflect"
	"testing"
	"time"
)

const qstatJobsJSON = `{
    "timestamp":1792374821,
    "pbs_version":"2024.1.0.20240130071604",
    "Jobs":{
        "675124.hopper-m-02":{
            "Job_Name":"g_spike",
            "Job_Owner":"libobo@hopper-l-01.cm.cluster",
            "job_state":"R",
            "queue":"small",
            "project":"cfp-1234",
            "qtime":"Mon Oct 19 09:10:00 2026",
            "exec_host":"hopper-04/0*12",
            "exec_vnode":"(hopper-04:ngpus=1:ncpus=12:mem=235929600kb)",
            "Resource_List":{"ngpus":1,"walltime":"01:30:00"},
            "resources_used":{"walltime":"00:26:10"}
        },
        "675200.hopper-m-02":{
            "Job_Name":"multi_node_training_run",
            "Job_Owner":"alice@hopper-l-02.cm.cluster",
            "job_state":"R",
            "queue":"large",
            "project":"cfp-5678",
            "exec_vnode":"(hopper-10:ngpus=4:ncpus=48)+(hopper-11:ncpus=48:ngpus=4)+(hopper-10:ngpus=2:ncpus=8)",
            "Resource_List":{"ngpus":10,"walltime":"406:46:16"},
            "resources_used":{"walltime":"100:00:00"}
        },
        "675118.hopper-m-02":{
            "Job_Name":"lmattn_p2",
            "Job_Owner":"dcsv481@hopper-l-01.cm.cluster",
            "job_state":"H",
            "queue":"interactive",
            "project":"cfp-1234",
            "qtime":"Fri Oct  2 11:06:24 2026",
            "Resource_List":{"ngpus":1,"walltime":"02:00:00"}
        },
        "675119.hopper-m-02":{
            "Job_Name":"queued_eval",
            "Job_Owner":"carol@hopper-l-03.cm.cluster",
            "job_state":"Q",
            "queue":"small",
            "qtime":1792374000,
            "Resource_List":{"ngpus":"2","walltime":"04:00:00"}
        },
        "675001.hopper-m-02":{
            "Job_Name":"done",
            "Job_Owner":"dave@hopper-l-01.cm.cluster",
            "job_state":"F",
            "queue":"small",
            "project":"cfp-1234"
        },
        "675300[1].hopper-m-02":{
            "Job_Name":"sweep",
            "Job_Owner":"bob@hopper-l-01.cm.cluster",
            "job_state":"R",
            "queue":"small",
            "project":"_pbs_project_default",
            "exec_vnode":"(hopper-20:ncpus=4)",
            "Resource_List":{"walltime":"00:10:00"},
            "resources_used":{"walltime":"00:00:05"}
        }
    }
}`

func TestParseQstatJobsJSON(t *testing.T) {
	jobs, version, err := NewClient().ParseQstatJobsJSON(qstatJobsJSON)
	if err != nil {
		t.Fatal(err)
	}
	if version != "2024.1.0" {
		t.Errorf("version = %q, want 2024.1.0", version)
	}

	byID := make(map[string]JobDetail)
	queuedAt := make(map[string]time.Time)
	for _, j := range jobs {
		queuedAt[j.JobID] = j.QueuedAt
		j.QueuedAt = time.Time{}
		byID[j.JobID] = j
	}

	want := map[string]JobDetail{
		"675124.hopper-m-02": {
			JobID: "675124.hopper-m-02", State: "R", User: "libobo", Queue: "small", Project: "cfp-1234", Name: "g_spike",
			Nodes: map[string]int{"hopper-04": 1}, NodeCPUs: map[string]int{"hopper-04": 12}, GPUsRequested: 1,
			WalltimeUsed: 26*60 + 10, WalltimeRequested: 90 * 60,
		},
		"675200.hopper-m-02": {
			JobID: "675200.hopper-m-02", State: "R", User: "alice", Queue: "large", Project: "cfp-5678", Name: "multi_node_training_run",
			Nodes: map[string]int{"hopper-10": 6, "hopper-11": 4}, NodeCPUs: map[string]int{"hopper-10": 56, "hopper-11": 48}, GPUsRequested: 10,
			WalltimeUsed: 100 * 3600, WalltimeRequested: 406*3600 + 46*60 + 16,
		},
		"675118.hopper-m-02": {
			JobID: "675118.hopper-m-02", State: "H", User: "dcsv481", Queue: "interactive", Project: "cfp-1234", Name: "lmattn_p2",
			GPUsRequested: 1, WalltimeRequested: 2 * 3600,
		},
		"675119.hopper-m-02": {
			JobID: "675119.hopper-m-02", State: "Q", User: "carol", Queue: "small", Project: "none", Name: "queued_eval",
			GPUsRequested: 2, WalltimeRequested: 4 * 3600,
		},
		"675001.hopper-m-02": {
			JobID: "675001.hopper-m-02", State: "F", User: "dave", Queue: "small", Project: "cfp-1234", Name: "done",
		},
		"675300[1].hopper-m-02": {
			JobID: "675300[1].hopper-m-02", State: "R", User: "bob", Queue: "small", Project: "_pbs_project_default", Name: "sweep",
			Nodes: map[string]int{"hopper-20": 0}, NodeCPUs: map[string]int{"hopper-20": 4},
			WalltimeUsed: 5, WalltimeRequested: 600,
		},
	}

	if !reflect.DeepEqual(byID, want) {
		t.Errorf("ParseQstatJobsJSON mismatch\n got: %+v\nwant: %+v", byID, want)
	}

	wantQueuedAt := map[string]time.Time{
		"675124.hopper-m-02":    time.Date(2026, 10, 19, 9, 10, 0, 0, time.Local),
		"675118.hopper-m-02":    time.Date(2026, 10, 2, 11, 6, 24, 0, time.Local),
		"675119.hopper-m-02":    time.Unix(1792374000, 0),
		"675200.hopper-m-02":    {},
		"675001.hopper-m-02":    {},
		"675300[1].hopper-m-02": {},
	}
	for id, w := range wantQueuedAt {
		if !queuedAt[id].Equal(w) {
			t.Errorf("%s QueuedAt = %v, want %v", id, queuedAt[id], w)
		}
	}
}

func TestParseQstatJobsJSONInvalid(t *testing.T) {
	if jobs, _, err := NewClient().ParseQstatJobsJSON("qstat: not json"); err == nil || jobs != nil {
		t.Errorf("expected error and no jobs for invalid JSON, got %+v, %v", jobs, err)
	}
}

func TestBuildJobData(t *testing.T) {
	jobs, _, err := NewClient().ParseQstatJobsJSON(qstatJobsJSON)
	if err != nil {
		t.Fatal(err)
	}
	got := BuildJobData(jobs)
	want := &JobData{
		UserJobCount:     map[string]int{"libobo": 1, "alice": 1, "bob": 1},
		QueuedJobsByUser: map[string]int{"carol": 1},
		QueueJobCount:    map[string]int{"small": 2, "large": 1},
		QueueTotalCount:  map[string]int{"small": 4, "large": 1, "interactive": 1},
		StatusCount:      map[string]int{"Running": 3, "Hold": 1, "Queuing": 1, "Finished": 1},
		TotalR:           3, TotalH: 1, TotalF: 1, TotalQ: 1,
		TotalAll: 6, TotalRunning: 3,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildJobData mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

// TestParseQstatJobsJSONFile parses a real `qstat -f -F json -t` dump when
// PBS_QSTAT_JSON points to one, e.g. PBS_QSTAT_JSON=/tmp/jobs.json go test ./...
func TestParseQstatJobsJSONFile(t *testing.T) {
	path := os.Getenv("PBS_QSTAT_JSON")
	if path == "" {
		t.Skip("PBS_QSTAT_JSON not set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	jobs, version, err := NewClient().ParseQstatJobsJSON(string(data))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("pbs version %s", version)
	if len(jobs) == 0 {
		t.Fatal("no running jobs parsed")
	}
	for _, j := range jobs {
		if j.User == "" || j.Name == "" || (j.State == "R" && len(j.Nodes) == 0) {
			t.Errorf("incomplete job: %+v", j)
		}
		t.Logf("%s %s %s %s %s %q %v gpus=%d", j.JobID, j.State, j.User, j.Queue, j.Project, j.Name, j.Nodes, j.GPUsRequested)
	}
}

const qstatQueuesJSON = `{
    "timestamp":1792374821,
    "pbs_version":"2024.1.0.20240130071604",
    "Queue":{
        "AISG_debug":{
            "queue_type":"Execution",
            "total_jobs":38,
            "state_count":"Transit:0 Queued:0 Held:1 Waiting:0 Running:32 Exiting:0 Begun:5 ",
            "resources_max":{"ngpus":128, "walltime":"1440:00:00"},
            "enabled":"True",
            "started":"True"
        },
        "smallx":{
            "queue_type":"Execution",
            "total_jobs":7,
            "state_count":"Transit:0 Queued:4 Held:0 Waiting:0 Running:3 Exiting:0 Begun:0 ",
            "resources_max":{"walltime":"24:00:00"},
            "enabled":"False",
            "started":"True"
        },
        "auto":{
            "queue_type":"Route",
            "total_jobs":0,
            "state_count":"Transit:0 Queued:0 Held:0 Waiting:0 Running:0 Exiting:0 Begun:0 ",
            "enabled":"True",
            "started":"True"
        }
    }
}`

func TestParseQstatQueuesJSON(t *testing.T) {
	got, err := NewClient().ParseQstatQueuesJSON(qstatQueuesJSON)
	if err != nil {
		t.Fatal(err)
	}
	zero := func(m map[string]int) map[string]int {
		for _, s := range []string{"Transit", "Queued", "Held", "Waiting", "Running", "Exiting", "Begun"} {
			if _, ok := m[s]; !ok {
				m[s] = 0
			}
		}
		return m
	}
	want := map[string]QueueInfo{
		"AISG_debug": {Type: "Execution", Enabled: true, Started: true, Walltime: 1440 * 3600, TotalJobs: 38,
			Jobs: zero(map[string]int{"Held": 1, "Running": 32, "Begun": 5})},
		"smallx": {Type: "Execution", Enabled: false, Started: true, Walltime: 24 * 3600, TotalJobs: 7,
			Jobs: zero(map[string]int{"Queued": 4, "Running": 3})},
		"auto": {Type: "Route", Enabled: true, Started: true, Jobs: zero(map[string]int{})},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseQstatQueuesJSON mismatch\n got: %+v\nwant: %+v", got, want)
	}

	if _, err := NewClient().ParseQstatQueuesJSON("not json"); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

const amJobCacheJSON = `{
    "pbs_version":"2026.0.0",
    "Jobs":{
        "1.srv":{
            "Job_Owner":"alice@login",
            "job_state":"R",
            "queue":"gpu",
            "exec_vnode":"(n1:ngpus=2:ncpus=24)",
            "resources_used":{
                "walltime":"01:30:00"
            },
            "Resource_List":{
                "am_job_cache":"{\"job.walltime\": 7200, \"job.ncpus\": 24.0, \"job.ngpus\": 2.0, \"job.queue_charge_rate\": 1.0, \"formula_constants\": \"\", \"am_cloud_enabled\": false}",
                "ngpus":2,
                "walltime":"02:00:00"
            }
        },
        "2.srv":{
            "Job_Owner":"bob@login",
            "job_state":"Q",
            "queue":"free",
            "Resource_List":{
                "am_job_cache":"{\"job.walltime\": 3600, \"job.ncpus\": 12.0, \"job.ngpus\": 1.0, \"job.queue_charge_rate\": 0}",
                "ngpus":1,
                "walltime":"01:00:00"
            }
        },
        "3.srv":{
            "Job_Owner":"carol@login",
            "job_state":"Q",
            "queue":"gpu",
            "Resource_List":{
                "am_job_cache":""{}\"",
                "ngpus":4,
                "walltime":"01:00:00"
            }
        },
        "4.srv":{
            "Job_Owner":"dave@login",
            "job_state":"Q",
            "queue":"gpu",
            "Resource_List":{
                "ngpus":1
            }
        }
    }
}`

func TestParseAmJobCache(t *testing.T) {
	jobs, version, err := NewClient().ParseQstatJobsJSON(amJobCacheJSON)
	if err != nil {
		t.Fatalf("malformed am_job_cache should not fail parsing: %v", err)
	}
	if version != "2026.0.0" {
		t.Errorf("version = %q", version)
	}
	byID := map[string]JobDetail{}
	for _, j := range jobs {
		byID[j.JobID] = j
	}
	if len(byID) != 4 {
		t.Fatalf("got %d jobs, want 4", len(byID))
	}

	b := byID["1.srv"].Budget
	if b == nil {
		t.Fatal("1.srv: budget not parsed")
	}
	if b.Walltime != 7200 || b.NCPUs != 24 || b.NGPUs != 2 || b.ChargeRate != 1 {
		t.Errorf("1.srv budget = %+v", *b)
	}
	if got := b.GPUHoursRequested(); got != 4 {
		t.Errorf("1.srv requested GPU-hours = %v, want 4", got)
	}
	if got := b.GPUHoursUsed(byID["1.srv"].WalltimeUsed); got != 3 {
		t.Errorf("1.srv used GPU-hours = %v, want 3", got)
	}

	free := byID["2.srv"].Budget
	if free == nil || free.ChargeRate != 0 || free.GPUHoursRequested() != 0 {
		t.Errorf("2.srv (free queue) budget = %+v", free)
	}
	if byID["3.srv"].Budget != nil {
		t.Errorf("3.srv: malformed am_job_cache should give nil budget, got %+v", *byID["3.srv"].Budget)
	}
	if byID["3.srv"].GPUsRequested != 4 {
		t.Errorf("3.srv: other fields should still parse, ngpus = %d", byID["3.srv"].GPUsRequested)
	}
	if byID["4.srv"].Budget != nil {
		t.Error("4.srv: no am_job_cache should give nil budget")
	}
}

const cpuJobsJSON = `{
    "pbs_version":"2026.0.0",
    "Jobs":{
        "10.srv":{
            "Job_Owner":"alice@login", "job_state":"R", "queue":"batch_cpu", "project":"p1",
            "exec_vnode":"(c1:ncpus=64:mem=262144000kb)+(c2:ncpus=32:mem=131072000kb)",
            "Resource_List":{ "ncpus":96, "mem":"375gb" }
        },
        "11.srv":{
            "Job_Owner":"bob@login", "job_state":"R", "queue":"gpu", "project":"p2",
            "exec_vnode":"(g1:ngpus=2:ncpus=24:mem=471859200kb)",
            "Resource_List":{ "ncpus":24, "ngpus":2, "mem":"450GB" }
        },
        "12.srv":{
            "Job_Owner":"carol@login", "job_state":"R", "queue":"batch_cpu",
            "exec_vnode":"(c3:mem=1gb)",
            "Resource_List":{ "ncpus":8, "mem":"1gb" }
        },
        "13.srv":{
            "Job_Owner":"dave@login", "job_state":"Q", "queue":"batch_cpu", "project":"p1",
            "Resource_List":{ "ncpus":"16", "mem":"64gb" }
        }
    }
}`

func TestParseJobCPUsAndMem(t *testing.T) {
	jobs, _, err := NewClient().ParseQstatJobsJSON(cpuJobsJSON)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]JobDetail{}
	for _, j := range jobs {
		byID[j.JobID] = j
	}

	multi := byID["10.srv"]
	if multi.NodeCPUs["c1"] != 64 || multi.NodeCPUs["c2"] != 32 || len(multi.NodeCPUs) != 2 {
		t.Errorf("10.srv NodeCPUs = %v", multi.NodeCPUs)
	}
	if multi.Nodes["c1"] != 0 || len(multi.Nodes) != 2 {
		t.Errorf("10.srv CPU-only job should still list nodes with 0 GPUs: %v", multi.Nodes)
	}
	if multi.CPUsRequested != 96 || multi.MemRequested != 375*(1<<30) {
		t.Errorf("10.srv requested ncpus=%d mem=%v", multi.CPUsRequested, multi.MemRequested)
	}
	if gpu := byID["11.srv"]; gpu.NodeCPUs["g1"] != 24 || gpu.Nodes["g1"] != 2 || gpu.MemRequested != 450*(1<<30) {
		t.Errorf("11.srv NodeCPUs=%v Nodes=%v mem=%v", gpu.NodeCPUs, gpu.Nodes, gpu.MemRequested)
	}
	if fb := byID["12.srv"]; fb.NodeCPUs["c3"] != 8 {
		t.Errorf("12.srv: exec_vnode without ncpus should fall back to Resource_List.ncpus, got %v", fb.NodeCPUs)
	}
	if q := byID["13.srv"]; q.CPUsRequested != 16 || q.NodeCPUs != nil || q.MemRequested != 64*(1<<30) {
		t.Errorf("13.srv queued ncpus=%d NodeCPUs=%v mem=%v", q.CPUsRequested, q.NodeCPUs, q.MemRequested)
	}
}

func TestParseMemoryToBytes(t *testing.T) {
	cases := map[string]float64{
		"450gb":       450 * (1 << 30),
		"100GB":       100 * (1 << 30),
		"471859200kb": 471859200 * 1024,
		"2tb":         2 * (1 << 40),
		"512mb":       512 * (1 << 20),
		"1024":        1024,
		"1024b":       1024,
		"1kw":         8 * 1024,
		"":            0,
		"--":          0,
	}
	for in, want := range cases {
		if got := parseMemoryToBytes(in); got != want {
			t.Errorf("parseMemoryToBytes(%q) = %v, want %v", in, got, want)
		}
	}
}

// Vanda-style vnode names (CN-001[0]) and a chunk spread over several vnodes
// inside one pair of parentheses
func TestParseExecVnodeVandaStyle(t *testing.T) {
	ev := "(CN-001[0]:ncpus=32:mem=128gb+CN-001[1]:ncpus=32:mem=128gb)+(CN-002[0]:ncpus=16:ngpus=1)"
	cpus := parseExecVnodeResource(ev, "ncpus")
	want := map[string]int{"CN-001[0]": 32, "CN-001[1]": 32, "CN-002[0]": 16}
	if !reflect.DeepEqual(cpus, want) {
		t.Errorf("ncpus = %v, want %v", cpus, want)
	}
	gpus := parseExecVnodeGPUs(ev)
	if gpus["CN-002[0]"] != 1 || gpus["CN-001[0]"] != 0 || len(gpus) != 3 {
		t.Errorf("ngpus = %v", gpus)
	}
}

func TestSubjobInheritsParentProject(t *testing.T) {
	const out = `{"Jobs":{
		"1550616[].srv":{"Job_Owner":"u1@h","job_state":"B","queue":"batch_cpu","project":"CFP03-CF-027"},
		"1550616[249].srv":{"Job_Owner":"u1@h","job_state":"Q","queue":"batch_cpu","Resource_List":{"ncpus":16}},
		"1550616[250].srv":{"Job_Owner":"u1@h","job_state":"R","queue":"batch_cpu","project":"own","exec_vnode":"(CN-001[0]:ncpus=16)"},
		"77[3].srv":{"Job_Owner":"u2@h","job_state":"Q","queue":"q"},
		"88.srv":{"Job_Owner":"u3@h","job_state":"Q","queue":"q"}
	}}`
	jobs, _, err := NewClient().ParseQstatJobsJSON(out)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, j := range jobs {
		got[j.JobID] = j.Project
	}
	want := map[string]string{
		"1550616[].srv":    "CFP03-CF-027",
		"1550616[249].srv": "CFP03-CF-027", // queued subjob without project: from parent
		"1550616[250].srv": "own",          // subjob's own project wins
		"77[3].srv":        "none",         // parent not in output
		"88.srv":           "none",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("projects = %v, want %v", got, want)
	}
	for in, want := range map[string]string{"123[4].srv": "123[].srv", "123[].srv": "", "123.srv": "", "x": ""} {
		if got := parentArrayID(in); got != want {
			t.Errorf("parentArrayID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParsePbsnodesJSON(t *testing.T) {
	const out = `{"pbs_version":"2026.0.0","nodes":{
		"gn-a40-074":{"state":"job-busy","comment":"failing job starts, checking","last_state_change_time":1795117200},
		"cn-018":{"state":"down,offline","comment":"node down: communication closed","last_state_change_time":"Fri Nov 20 09:00:00 2026"},
		"cn-001":{"state":"free","comment":" "},
		"cn-002":{"state":"free"}
	}}`
	nodes, err := NewClient().ParsePbsnodesJSON(out)
	if err != nil {
		t.Fatal(err)
	}
	if n := nodes["gn-a40-074"]; n.State != "job-busy" || n.Comment != "failing job starts, checking" || n.LastStateChange.Unix() != 1795117200 {
		t.Errorf("gn-a40-074 = %+v", n)
	}
	if n := nodes["cn-018"]; n.State != "down,offline" || n.Comment != "node down: communication closed" || n.LastStateChange.IsZero() {
		t.Errorf("cn-018 = %+v", n)
	}
	if nodes["cn-001"].Comment != "" || nodes["cn-002"].Comment != "" || len(nodes) != 4 {
		t.Errorf("blank comments should be empty: %+v", nodes)
	}
	if _, err := NewClient().ParsePbsnodesJSON("not json"); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

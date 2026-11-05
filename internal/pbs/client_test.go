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
			Nodes: map[string]int{"hopper-04": 1}, GPUsRequested: 1,
			WalltimeUsed: 26*60 + 10, WalltimeRequested: 90 * 60,
		},
		"675200.hopper-m-02": {
			JobID: "675200.hopper-m-02", State: "R", User: "alice", Queue: "large", Project: "cfp-5678", Name: "multi_node_training_run",
			Nodes: map[string]int{"hopper-10": 6, "hopper-11": 4}, GPUsRequested: 10,
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
			Nodes:        map[string]int{"hopper-20": 0},
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

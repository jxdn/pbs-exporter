package pbs

import (
	"os"
	"reflect"
	"testing"
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
            "exec_vnode":"(hopper-10:ngpus=4:ncpus=48)+(hopper-11:ncpus=48:ngpus=4)+(hopper-10:ngpus=2:ncpus=8)",
            "Resource_List":{"ngpus":10,"walltime":"406:46:16"},
            "resources_used":{"walltime":"100:00:00"}
        },
        "675118.hopper-m-02":{
            "Job_Name":"lmattn_p2",
            "Job_Owner":"dcsv481@hopper-l-01.cm.cluster",
            "job_state":"H",
            "queue":"interactive",
            "Resource_List":{"ngpus":1,"walltime":"02:00:00"}
        },
        "675300[1].hopper-m-02":{
            "Job_Name":"sweep",
            "Job_Owner":"bob@hopper-l-01.cm.cluster",
            "job_state":"R",
            "queue":"small",
            "exec_vnode":"(hopper-20:ncpus=4)",
            "Resource_List":{"walltime":"00:10:00"},
            "resources_used":{"walltime":"00:00:05"}
        }
    }
}`

func TestParseQstatJobsJSON(t *testing.T) {
	jobs := NewClient().ParseQstatJobsJSON(qstatJobsJSON)

	byID := make(map[string]JobDetail)
	for _, j := range jobs {
		byID[j.JobID] = j
	}

	want := map[string]JobDetail{
		"675124.hopper-m-02": {
			JobID: "675124.hopper-m-02", User: "libobo", Queue: "small", Name: "g_spike",
			Nodes:        map[string]int{"hopper-04": 1},
			WalltimeUsed: 26*60 + 10, WalltimeRequested: 90 * 60,
		},
		"675200.hopper-m-02": {
			JobID: "675200.hopper-m-02", User: "alice", Queue: "large", Name: "multi_node_training_run",
			Nodes:        map[string]int{"hopper-10": 6, "hopper-11": 4},
			WalltimeUsed: 100 * 3600, WalltimeRequested: 406*3600 + 46*60 + 16,
		},
		"675300[1].hopper-m-02": {
			JobID: "675300[1].hopper-m-02", User: "bob", Queue: "small", Name: "sweep",
			Nodes:        map[string]int{"hopper-20": 0},
			WalltimeUsed: 5, WalltimeRequested: 600,
		},
	}

	if !reflect.DeepEqual(byID, want) {
		t.Errorf("ParseQstatJobsJSON mismatch\n got: %+v\nwant: %+v", byID, want)
	}
}

func TestParseQstatJobsJSONInvalid(t *testing.T) {
	if jobs := NewClient().ParseQstatJobsJSON("qstat: not json"); jobs != nil {
		t.Errorf("expected nil for invalid JSON, got %+v", jobs)
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
	jobs := NewClient().ParseQstatJobsJSON(string(data))
	if len(jobs) == 0 {
		t.Fatal("no running jobs parsed")
	}
	for _, j := range jobs {
		if j.User == "" || j.Name == "" || len(j.Nodes) == 0 {
			t.Errorf("incomplete job: %+v", j)
		}
		t.Logf("%s %s %s %q %v used=%ds", j.JobID, j.User, j.Queue, j.Name, j.Nodes, j.WalltimeUsed)
	}
}

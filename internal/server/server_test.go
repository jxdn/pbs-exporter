package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"pbs-exporter/internal/metrics"
	"pbs-exporter/internal/pbs"
)

// fakeQstat puts a `qstat` script on PATH that prints output
func fakeQstat(t *testing.T, output string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "out.json"), []byte(output), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncat " + filepath.Join(dir, "out.json") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "qstat"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestQueuedJobGPUs(t *testing.T) {
	fakeQstat(t, `{
    "pbs_version":"2024.1.0",
    "Jobs":{
        "1.srv":{"Job_Name":"gpu","Job_Owner":"alice@login","job_state":"Q","queue":"AISG_debug","project":"AISG",
            "qtime":"Mon Nov 23 10:00:00 2026","Resource_List":{"ncpus":24,"ngpus":2,"walltime":"01:00:00"}},
        "2.srv":{"Job_Name":"held","Job_Owner":"bob@login","job_state":"H","Hold_Types":"u","queue":"AISG_large","project":"AISG",
            "qtime":"Mon Nov 23 10:00:00 2026","Resource_List":{"ncpus":96,"ngpus":8,"walltime":"01:00:00"}},
        "3.srv":{"Job_Name":"cpu","Job_Owner":"carol@login","job_state":"Q","queue":"cpu","project":"P1",
            "qtime":"Mon Nov 23 10:00:00 2026","Resource_List":{"ncpus":4,"walltime":"01:00:00"}},
        "4.srv":{"Job_Name":"run","Job_Owner":"dave@login","job_state":"R","queue":"AISG_debug","project":"AISG",
            "exec_vnode":"(hopper-31:ngpus=1:ncpus=12)","Resource_List":{"ncpus":12,"ngpus":1,"walltime":"01:00:00"},
            "resources_used":{"walltime":"00:10:00"}}
    }
}`)
	r := metrics.NewRegistry()
	s := New(r, pbs.NewClient())
	s.updateJobMetrics()

	if got := testutil.ToFloat64(r.ExporterCommandSuccess.WithLabelValues("qstat_json")); got != 1 {
		t.Fatalf("qstat_json success = %v, want 1", got)
	}
	// running jobs are not queued: only the 3 waiting jobs have a series.
	// Counted first, because WithLabelValues below creates missing series.
	if n := testutil.CollectAndCount(r.JobQueuedNgpus); n != 3 {
		t.Errorf("pbs_job_queued_ngpus has %d series, want 3", n)
	}
	want := map[string]float64{"1.srv": 2, "2.srv": 8, "3.srv": 0}
	for id, gpus := range want {
		if got := testutil.ToFloat64(r.JobQueuedNgpus.WithLabelValues(id)); got != gpus {
			t.Errorf("pbs_job_queued_ngpus{job_id=%q} = %v, want %v", id, got, gpus)
		}
	}
}

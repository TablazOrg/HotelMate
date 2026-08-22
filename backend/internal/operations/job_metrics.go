package operations

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

var metricJobName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
var lastSuccessMetric = regexp.MustCompile(`hotelmate_job_last_success_timestamp_seconds\{job="[^"]+"\}\s+([0-9]+)`)

func recordJobMetric(directory, job string, success bool, at time.Time) error {
	if directory == "" {
		return nil
	}
	if !metricJobName.MatchString(job) {
		return fmt.Errorf("invalid metric job name")
	}
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return err
	}
	value, lastSuccess := 0, int64(0)
	if success {
		value = 1
		lastSuccess = at.UTC().Unix()
	} else if existing, err := os.ReadFile(filepath.Join(directory, "hotelmate_"+job+".prom")); err == nil {
		match := lastSuccessMetric.FindSubmatch(existing)
		if len(match) == 2 {
			lastSuccess, _ = strconv.ParseInt(string(match[1]), 10, 64)
		}
	}
	content := fmt.Sprintf("# HELP hotelmate_job_last_run_success Whether the last job run succeeded.\n# TYPE hotelmate_job_last_run_success gauge\nhotelmate_job_last_run_success{job=%q} %d\n# HELP hotelmate_job_last_run_timestamp_seconds Unix timestamp of the last attempted job run.\n# TYPE hotelmate_job_last_run_timestamp_seconds gauge\nhotelmate_job_last_run_timestamp_seconds{job=%q} %d\n", job, value, job, at.UTC().Unix())
	if lastSuccess > 0 {
		content += fmt.Sprintf("# HELP hotelmate_job_last_success_timestamp_seconds Unix timestamp of the last successful job run.\n# TYPE hotelmate_job_last_success_timestamp_seconds gauge\nhotelmate_job_last_success_timestamp_seconds{job=%q} %d\n", job, lastSuccess)
	}
	path := filepath.Join(directory, "hotelmate_"+job+".prom")
	temporary, err := os.CreateTemp(directory, ".hotelmate-metric-*.partial")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o640); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.WriteString(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

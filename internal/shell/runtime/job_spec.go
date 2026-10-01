package runtime

import (
	"sort"
	"strconv"
	"strings"
)

// Job specs, as both references read them: %N is job N; %%, %+ and a lone % are the current
// job, the one started last of those the table still has; %- is the previous one, started
// before it. A job leaves the table once its end is reported, so when the current job is
// waited for, the previous one becomes current. %% and %- were "invalid job".
//
// %name and %?text name a job by the command it runs, and a command here keeps no written
// form to match -- busybox-w32 has none in a script either -- so they name no job.

// resolveJobSpec is the job a spec names, and 0 for none.
func (s *jobScope) resolveJobSpec(spec string) jobID {
	switch spec {
	case "%", "%%", "%+":
		current, _ := s.currentAndPrevious()
		return current
	case "%-":
		_, previous := s.currentAndPrevious()
		return previous
	}
	value, err := strconv.ParseUint(strings.TrimPrefix(spec, "%"), 10, 64)
	if err != nil {
		return 0
	}
	return jobID(value)
}

// noSuchJob is what kill, jobs and wait say of a spec that names no job: "No current job" and
// "No previous job" for those two, as every ash says them, busybox's and dash among them, and
// "%9: no such job" for the rest. `kill %%` with no jobs said "%%: no such job".
func noSuchJob(spec string) string {
	switch spec {
	case "%", "%%", "%+":
		return "No current job"
	case "%-":
		return "No previous job"
	}
	return spec + ": no such job"
}

// snapshot is the table newest first, the order busybox lists it in: the current job, then
// the previous one, then the rest.
func (s *jobScope) snapshot() []*jobRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := make([]*jobRecord, 0, len(s.records))
	for _, record := range s.records {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].started > records[j].started })
	return records
}

// currentAndPrevious are the jobs started last and before it, or 0 for either there is not.
func (s *jobScope) currentAndPrevious() (jobID, jobID) {
	records := s.snapshot()
	var current, previous jobID
	if len(records) > 0 {
		current = records[0].id
	}
	if len(records) > 1 {
		previous = records[1].id
	}
	return current, previous
}

// jobMarker is the mark `jobs` puts after a job's number: + for the current job, - for the
// previous one, and a blank for the rest.
func jobMarker(id, current, previous jobID) byte {
	switch id {
	case current:
		return '+'
	case previous:
		return '-'
	}
	return ' '
}

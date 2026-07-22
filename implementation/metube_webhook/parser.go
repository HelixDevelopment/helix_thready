package metubewebhook

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// wireJob is the on-the-wire shape of one MeTube postprocess job, as returned
// by GET /api/postprocess/jobs. Fields are lenient: MeTube variants report the
// error text under either "error" or "msg", and percent may be absent.
type wireJob struct {
	ID       string   `json:"id"`
	Status   string   `json:"status"`
	Percent  *float64 `json:"percent"`
	Filename string   `json:"filename"`
	Error    string   `json:"error"`
	Msg      string   `json:"msg"`
}

// wireEnvelope is the object form of the response: {"jobs": [ ... ]}.
type wireEnvelope struct {
	Jobs []wireJob `json:"jobs"`
}

// ParseJobs maps a raw MeTube /api/postprocess/jobs JSON body into normalized
// JobStatus values. It accepts either the object form {"jobs":[...]} or a bare
// top-level array [...]. It is pure and offline-testable.
func ParseJobs(data []byte) ([]JobStatus, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("metubewebhook: empty status body")
	}

	var wires []wireJob
	switch trimmed[0] {
	case '[':
		if err := json.Unmarshal(trimmed, &wires); err != nil {
			return nil, fmt.Errorf("metubewebhook: decode job array: %w", err)
		}
	case '{':
		var env wireEnvelope
		if err := json.Unmarshal(trimmed, &env); err != nil {
			return nil, fmt.Errorf("metubewebhook: decode job object: %w", err)
		}
		wires = env.Jobs
	default:
		return nil, fmt.Errorf("metubewebhook: unrecognized status JSON (starts with %q)", trimmed[0])
	}

	out := make([]JobStatus, 0, len(wires))
	for _, w := range wires {
		out = append(out, mapWireJob(w))
	}
	return out, nil
}

// mapWireJob normalizes a single wire job into a JobStatus:
//   - percent (0..100) -> progress (0.0..1.0), clamped; a finished job with no
//     reported percent is treated as fully complete (1.0).
//   - the error text is taken from "error", falling back to "msg".
func mapWireJob(w wireJob) JobStatus {
	state := JobState(w.Status)

	progress := 0.0
	if w.Percent != nil {
		progress = *w.Percent / 100.0
	} else if state == StateFinished {
		progress = 1.0
	}
	progress = clamp01(progress)
	if state == StateFinished {
		progress = 1.0
	}

	errMsg := w.Error
	if errMsg == "" {
		errMsg = w.Msg
	}

	return JobStatus{
		ID:         w.ID,
		State:      state,
		Progress:   progress,
		ResultPath: w.Filename,
		Error:      errMsg,
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

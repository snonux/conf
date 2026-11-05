package recovery

// Status is the explicitly public subset of the private transaction journal.
// It contains no network objects or authentication/USB/video configuration.
type Status struct {
	Device     string `json:"device"`
	Generation string `json:"generation"`
	Phase      string `json:"phase"`
	Boot       string `json:"boot"`
	Deadline   int64  `json:"deadline_boottime_ns"`
	Allowance  bool   `json:"allowance"`
	Fallback   bool   `json:"fallback_used"`
}

// Status reads a generation/device-checked public summary under the shared lock.
func (e Engine) Status(generation string) (Status, error) {
	var status Status
	err := e.Store.Locked(func() error {
		state, err := e.checked(generation)
		if err != nil {
			return err
		}
		status = Status{state.Device, state.Generation, state.Phase, state.Boot, state.Deadline, state.Allowance, state.Fallback}
		return nil
	})
	return status, err
}

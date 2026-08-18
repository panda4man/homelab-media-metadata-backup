package validation

// State is the outcome of a single inventory run.
type State int

const (
	StateValid State = iota
	StateWarning
	StateFailed
)

func (s State) String() string {
	switch s {
	case StateValid:
		return "valid"
	case StateWarning:
		return "warning"
	case StateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// fatalCheckIDs are the checks that mean the run cannot be trusted at all:
// an unreachable Arr, an inaccessible or unexpectedly empty media root, or
// a scan that terminated early. Everything else is a content-level
// threshold breach - a real but survivable change, resolved as a warning.
var fatalCheckIDs = map[string]bool{
	"sonarr_unreachable":      true,
	"radarr_unreachable":      true,
	"media_root_inaccessible": true,
	"media_root_empty":        true,
	"scan_incomplete":         true,
}

// Resolve maps evaluated Checks to a State. Any triggered fatal check
// forces StateFailed even if warning-level checks also triggered; any
// remaining triggered check forces StateWarning; otherwise StateValid.
//
// Resolve deliberately takes only checks - not InfluxDB status, not
// off-site upload status. Those are separate operational concerns (see
// the orchestrator, a later slice) and must never influence whether a
// snapshot is trustworthy as a disaster-recovery record.
func Resolve(checks []Check) State {
	sawWarning := false
	for _, c := range checks {
		if !c.Triggered {
			continue
		}
		if fatalCheckIDs[c.ID] {
			return StateFailed
		}
		sawWarning = true
	}
	if sawWarning {
		return StateWarning
	}
	return StateValid
}

package report

import (
	"fmt"
	"strconv"
	"strings"
)

// Prometheus renders the report as a Prometheus/OpenMetrics text exposition,
// suitable for the node_exporter textfile collector (write to a *.prom file).
// Every series is a gauge (a per-scan snapshot) and carries no timestamp, per
// exposition-format conventions; the collector assigns the scrape time and a
// separate node_textfile_mtime_seconds tracks staleness.
func (r Report) Prometheus() []byte {
	var b strings.Builder

	fmt.Fprintf(&b, "# HELP regionlock_up 1 if the evidence report was generated successfully.\n")
	fmt.Fprintf(&b, "# TYPE regionlock_up gauge\n")
	fmt.Fprintf(&b, "regionlock_up 1\n")

	fmt.Fprintf(&b, "# HELP regionlock_compliance_ratio Fraction of checks passing (0-1).\n")
	fmt.Fprintf(&b, "# TYPE regionlock_compliance_ratio gauge\n")
	fmt.Fprintf(&b, "regionlock_compliance_ratio{ruleset=%s,jurisdiction=%s} %s\n",
		promLabel(r.Ruleset.ID), promLabel(r.Ruleset.Jurisdiction),
		strconv.FormatFloat(r.Summary.Score/100, 'f', -1, 64))

	fmt.Fprintf(&b, "# HELP regionlock_checks Checks by status in the latest scan.\n")
	fmt.Fprintf(&b, "# TYPE regionlock_checks gauge\n")
	fmt.Fprintf(&b, "regionlock_checks{status=%s} %d\n", promLabel("pass"), r.Summary.Pass)
	fmt.Fprintf(&b, "regionlock_checks{status=%s} %d\n", promLabel("fail"), r.Summary.Fail)
	fmt.Fprintf(&b, "regionlock_checks{status=%s} %d\n", promLabel("skip"), r.Summary.Skip)

	fmt.Fprintf(&b, "# HELP regionlock_violations Failing checks by control and severity in the latest scan.\n")
	fmt.Fprintf(&b, "# TYPE regionlock_violations gauge\n")
	for _, rc := range r.RuleScores {
		fmt.Fprintf(&b, "regionlock_violations{rule=%s,severity=%s} %d\n",
			promLabel(rc.RuleID), promLabel(rc.Severity), rc.Fail)
	}

	fmt.Fprintf(&b, "# HELP regionlock_resources Distinct resources evaluated in the latest scan.\n")
	fmt.Fprintf(&b, "# TYPE regionlock_resources gauge\n")
	fmt.Fprintf(&b, "regionlock_resources %d\n", r.Summary.Resources)

	fmt.Fprintf(&b, "# HELP regionlock_report_build_info Report context; value is always 1.\n")
	fmt.Fprintf(&b, "# TYPE regionlock_report_build_info gauge\n")
	fmt.Fprintf(&b, "regionlock_report_build_info{tool=%s,version=%s,ruleset=%s,ruleset_version=%s,jurisdiction=%s} 1\n",
		promLabel(r.Tool), promLabel(r.Version), promLabel(r.Ruleset.ID),
		promLabel(r.Ruleset.Version), promLabel(r.Ruleset.Jurisdiction))

	return []byte(b.String())
}

// promLabel returns a quoted, escaped Prometheus label value. The exposition
// format requires backslash, double-quote and line-feed to be escaped.
func promLabel(v string) string {
	rep := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + rep.Replace(v) + `"`
}

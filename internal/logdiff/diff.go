package logdiff

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

type Capture struct {
	Entries      []*logentry.Entry
	ProcessByPID map[int]string
	PackageByPID map[int]string
}

type ErrorSignature struct {
	Level   string
	Tag     string
	Process string
	Package string
	Message string
	Count   int
}

type CountDelta struct {
	Name      string
	Baseline  int
	Candidate int
	Delta     int
}

type Report struct {
	NewErrors []ErrorSignature
	Tags      []CountDelta
	Processes []CountDelta
	Packages  []CountDelta
}

func ReadCapture(path string) (Capture, error) {
	f, err := os.Open(path)
	if err != nil {
		return Capture{}, fmt.Errorf("打开日志文件失败: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var entries []*logentry.Entry
	for scanner.Scan() {
		entry := logentry.Parse(scanner.Text())
		if entry != nil {
			entries = append(entries, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return Capture{}, err
	}
	return Capture{Entries: entries}, nil
}

func CompareCaptures(base, candidate Capture) Report {
	return Report{
		NewErrors: newErrorSignatures(base, candidate),
		Tags:      changedCounts(tagCounts(base), tagCounts(candidate)),
		Processes: changedCounts(processCounts(base), processCounts(candidate)),
		Packages:  changedCounts(packageCounts(base), packageCounts(candidate)),
	}
}

func FormatReport(report Report) string {
	var sb strings.Builder
	sb.WriteString("LogCaTool diff\n\n")
	sb.WriteString("New error/fatal signatures:\n")
	if len(report.NewErrors) == 0 {
		sb.WriteString("  none\n")
	} else {
		for _, sig := range report.NewErrors {
			context := sigContext(sig)
			if context != "" {
				context = " [" + context + "]"
			}
			sb.WriteString(fmt.Sprintf("  - %s %s%s %s (x%d)\n", sig.Level, sig.Tag, context, sig.Message, sig.Count))
		}
	}

	writeDeltaSection(&sb, "Changed tags", report.Tags)
	writeDeltaSection(&sb, "Changed processes", report.Processes)
	writeDeltaSection(&sb, "Changed packages", report.Packages)
	return sb.String()
}

func newErrorSignatures(base, candidate Capture) []ErrorSignature {
	baseCounts := errorSignatureCounts(base)
	candidateCounts := errorSignatureCounts(candidate)

	keys := make([]string, 0, len(candidateCounts))
	for key := range candidateCounts {
		if baseCounts[key].count == 0 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	out := make([]ErrorSignature, 0, len(keys))
	for _, key := range keys {
		sig := candidateCounts[key].signature
		sig.Count = candidateCounts[key].count
		out = append(out, sig)
	}
	return out
}

type countedSignature struct {
	signature ErrorSignature
	count     int
}

func errorSignatureCounts(c Capture) map[string]countedSignature {
	counts := make(map[string]countedSignature)
	for _, entry := range c.Entries {
		if entry == nil || entry.Level < logentry.LevelError || entry.Level > logentry.LevelFatal {
			continue
		}
		sig := ErrorSignature{
			Level:   entry.Level.Char(),
			Tag:     entry.Tag,
			Process: processForEntry(c, entry),
			Package: packageForEntry(c, entry),
			Message: entry.Message,
		}
		key := signatureKey(sig)
		current := counts[key]
		current.signature = sig
		current.count++
		counts[key] = current
	}
	return counts
}

func signatureKey(sig ErrorSignature) string {
	return strings.Join([]string{sig.Level, sig.Tag, sig.Process, sig.Package, sig.Message}, "\x00")
}

func tagCounts(c Capture) map[string]int {
	counts := make(map[string]int)
	for _, entry := range c.Entries {
		if entry != nil && entry.Tag != "" {
			counts[entry.Tag]++
		}
	}
	return counts
}

func processCounts(c Capture) map[string]int {
	counts := make(map[string]int)
	for _, entry := range c.Entries {
		if process := processForEntry(c, entry); process != "" {
			counts[process]++
		}
	}
	return counts
}

func packageCounts(c Capture) map[string]int {
	counts := make(map[string]int)
	for _, entry := range c.Entries {
		if pkg := packageForEntry(c, entry); pkg != "" {
			counts[pkg]++
		}
	}
	return counts
}

func processForEntry(c Capture, entry *logentry.Entry) string {
	if entry == nil {
		return ""
	}
	if process := c.ProcessByPID[entry.PID]; process != "" {
		return process
	}
	return c.PackageByPID[entry.PID]
}

func packageForEntry(c Capture, entry *logentry.Entry) string {
	if entry == nil {
		return ""
	}
	if process := c.ProcessByPID[entry.PID]; process != "" {
		return packageNameFromProcess(process)
	}
	return c.PackageByPID[entry.PID]
}

func packageNameFromProcess(name string) string {
	if idx := strings.IndexByte(name, ':'); idx >= 0 {
		return name[:idx]
	}
	return name
}

func changedCounts(base, candidate map[string]int) []CountDelta {
	seen := make(map[string]bool, len(base)+len(candidate))
	for name := range base {
		seen[name] = true
	}
	for name := range candidate {
		seen[name] = true
	}

	out := make([]CountDelta, 0, len(seen))
	for name := range seen {
		if base[name] == candidate[name] {
			continue
		}
		out = append(out, CountDelta{
			Name:      name,
			Baseline:  base[name],
			Candidate: candidate[name],
			Delta:     candidate[name] - base[name],
		})
	}
	sort.Slice(out, func(i, j int) bool {
		absI := abs(out[i].Delta)
		absJ := abs(out[j].Delta)
		if absI != absJ {
			return absI > absJ
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func writeDeltaSection(sb *strings.Builder, title string, deltas []CountDelta) {
	sb.WriteString("\n")
	sb.WriteString(title)
	sb.WriteString(":\n")
	if len(deltas) == 0 {
		sb.WriteString("  none\n")
		return
	}
	for _, delta := range deltas {
		sb.WriteString(fmt.Sprintf("  - %s: %d -> %d (%+d)\n", delta.Name, delta.Baseline, delta.Candidate, delta.Delta))
	}
}

func sigContext(sig ErrorSignature) string {
	switch {
	case sig.Process != "" && sig.Package != "" && sig.Process != sig.Package:
		return sig.Process + " / " + sig.Package
	case sig.Process != "":
		return sig.Process
	case sig.Package != "":
		return sig.Package
	default:
		return ""
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// wakebench compares the cold start of the working tree with a release: it builds both
// binaries, then runs `--mode dry-run` on offline settings (local calendar and text files,
// no network) alternately and reports the median wall time of each.
//
// The dry run includes writing a PNG, which a panel does not; the overhead is the same for
// both binaries, so the difference is what counts. Absolute numbers are this machine's;
// on the device, the "cycle done" log line has the real timings.
//
//	go run ./tools/wakebench -base v0.1.2 -n 30
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func main() {
	base := flag.String("base", "", "git ref to compare with (default: the latest v* tag)")
	n := flag.Int("n", 20, "runs per binary")
	settings := flag.String("settings", "tools/wakebench/settings.json", "offline settings for the dry run")
	maxSlower := flag.Float64("max-slower", 0, "fail when the working tree's median is more than this many percent slower (0 = report only)")
	flag.Parse()
	if err := run(*base, *n, *settings, *maxSlower); err != nil {
		fmt.Fprintln(os.Stderr, "wakebench:", err)
		os.Exit(1)
	}
}

func run(base string, n int, settings string, maxSlower float64) error {
	if base == "" {
		out, err := exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", "v*").Output()
		if err != nil {
			return fmt.Errorf("no v* tag to compare with; pass -base: %w", err)
		}
		base = strings.TrimSpace(string(out))
	}
	tmp, err := os.MkdirTemp("", "wakebench-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	oldBin, newBin := filepath.Join(tmp, "old"), filepath.Join(tmp, "new")
	tree := filepath.Join(tmp, "tree")
	if err := sh("", "git", "worktree", "add", "--detach", "--quiet", tree, base); err != nil {
		return err
	}
	defer sh("", "git", "worktree", "remove", "--force", tree)
	if err := build(tree, oldBin); err != nil {
		return fmt.Errorf("build %s: %w", base, err)
	}
	if err := build(".", newBin); err != nil {
		return fmt.Errorf("build working tree: %w", err)
	}

	// Alternate the binaries so drift (thermal, other load) hits both alike.
	var oldT, newT []time.Duration
	for i := range n + 1 {
		o, err := dryRun(oldBin, settings, tmp)
		if err != nil {
			return fmt.Errorf("%s: %w", base, err)
		}
		w, err := dryRun(newBin, settings, tmp)
		if err != nil {
			return fmt.Errorf("working tree: %w", err)
		}
		if i > 0 { // the first round warms the page cache
			oldT, newT = append(oldT, o), append(newT, w)
		}
	}
	om, nm := median(oldT), median(newT)
	delta := 100 * (float64(nm) - float64(om)) / float64(om)
	fmt.Printf("%-14s median %7s  (min %s, max %s)\n", base, om.Round(time.Millisecond/10), slices.Min(oldT).Round(time.Millisecond/10), slices.Max(oldT).Round(time.Millisecond/10))
	fmt.Printf("%-14s median %7s  (min %s, max %s)\n", "working tree", nm.Round(time.Millisecond/10), slices.Min(newT).Round(time.Millisecond/10), slices.Max(newT).Round(time.Millisecond/10))
	fmt.Printf("difference     %+.1f%%  (%d runs each)\n", delta, n)
	if maxSlower > 0 && delta > maxSlower {
		return fmt.Errorf("working tree is %.1f%% slower than %s (limit %.1f%%)", delta, base, maxSlower)
	}
	return nil
}

func build(dir, out string) error {
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w", "-o", out, "./cmd/inkycal")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func dryRun(bin, settings, tmp string) (time.Duration, error) {
	state, err := os.MkdirTemp(tmp, "state-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(state)
	cmd := exec.Command(bin, "--mode", "dry-run", "--settings", settings,
		"--out", filepath.Join(state, "out.png"), "--state-dir", state)
	start := time.Now()
	out, err := cmd.CombinedOutput()
	d := time.Since(start)
	if err != nil {
		return 0, fmt.Errorf("%w\n%s", err, out)
	}
	if !strings.Contains(string(out), "module_errors=0") {
		return 0, fmt.Errorf("modules failed, the timing would not be comparable:\n%s", out)
	}
	return d, nil
}

func median(ds []time.Duration) time.Duration {
	s := slices.Clone(ds)
	slices.Sort(s)
	return s[len(s)/2]
}

func sh(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

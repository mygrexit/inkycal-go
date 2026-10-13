package deploy

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// install.sh embeds the systemd unit (it runs from a pipe and has no checkout);
// it must stay identical to deploy/inkycal-go.service.
func TestInstallerUnitMatchesServiceFile(t *testing.T) {
	script, err := os.ReadFile("install.sh")
	if err != nil {
		t.Fatal(err)
	}
	unit, err := os.ReadFile("inkycal-go.service")
	if err != nil {
		t.Fatal(err)
	}
	const start = "cat > /etc/systemd/system/inkycal-go.service <<'EOF'\n"
	i := strings.Index(string(script), start)
	if i < 0 {
		t.Fatal("unit heredoc not found in install.sh")
	}
	rest := string(script)[i+len(start):]
	embedded := rest[:strings.Index(rest, "EOF\n")]
	if embedded != string(unit) {
		t.Errorf("install.sh unit differs from inkycal-go.service:\n%s\n---\n%s", embedded, unit)
	}
}

// --dev installs the rolling "dev" pre-release; without it the latest stable release.
// GitHub never treats a pre-release as "latest", so stable users never get dev builds.
func TestInstallerChannels(t *testing.T) {
	script, err := os.ReadFile("install.sh")
	if err != nil {
		t.Fatal(err)
	}
	s := string(script)
	for _, want := range []string{
		"--dev) channel=dev ;;",
		`base="https://github.com/$REPO/releases/download/dev"`,
		`base="https://github.com/$REPO/releases/latest/download"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("install.sh lacks %s", want)
		}
	}
	if out, err := exec.Command("sh", "-n", "install.sh").CombinedOutput(); err != nil {
		t.Errorf("syntax: %v %s", err, out)
	}
}

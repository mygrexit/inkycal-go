package deploy

import (
	"os"
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

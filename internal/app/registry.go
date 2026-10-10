package app

import (
	"inkycal/internal/modules"
	"inkycal/internal/modules/agenda"
)

// Registry maps settings.json module names (as in Python's InkycalModuleImporter) to factories.
var Registry = map[string]modules.Factory{
	"Agenda": agenda.New,
}

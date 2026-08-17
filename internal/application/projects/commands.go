// Package projects hosts the application handlers of the Projects
// bounded context.
package projects

import (
	"context"
	"errors"
	"strings"

	appcmd "github.com/Rubentxu/golem/internal/application/command"
	"github.com/Rubentxu/golem/internal/ports"
	domainprojects "github.com/Rubentxu/golem/internal/projects"
	"github.com/Rubentxu/golem/internal/work"
)

// Domain validation errors of the Projects context.
var (
	ErrEmptyName = errors.New("projects: name is mandatory")
)

// Command names of this context.
const (
	CmdCreateProject = "projects.create-project"
)

// CreateProject is the payload of CmdCreateProject. ProjectID and
// External are importer-only escapes (stable id + provider identity).
type CreateProject struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	ProjectID   string                `json:"project_id,omitempty"`
	External    work.ExternalIdentity `json:"external,omitempty"`
}

// CreateProjectHandler returns the handler for CmdCreateProject.
func CreateProjectHandler(gen ports.IDGenerator) appcmd.Handler {
	return func(_ context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(CreateProject)
		if !ok {
			return nil, errors.New("projects: payload must be projects.CreateProject")
		}
		name := strings.TrimSpace(p.Name)
		if name == "" {
			return nil, ErrEmptyName
		}
		id := strings.TrimSpace(p.ProjectID)
		if id == "" {
			id = gen.NewID()
		}
		return []appcmd.EventDraft{{
			EventType:     domainprojects.EventProjectCreated,
			StreamID:      "project:" + id,
			SchemaVersion: 1,
			Payload: domainprojects.ProjectCreated{
				ProjectID: id, Name: name, Description: strings.TrimSpace(p.Description),
				External: p.External,
			},
		}}, nil
	}
}

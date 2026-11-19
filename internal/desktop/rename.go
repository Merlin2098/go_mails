package desktop

import (
	"log/slog"
	"path/filepath"
	"sync"

	"despacho/app/application"
)

type RenameRequest struct {
	Folder       string `json:"folder"`
	SourceColumn string `json:"sourceColumn"`
	Rule         string `json:"rule"`
}

type RenameOpDTO struct {
	Row    int    `json:"row"`
	From   string `json:"from"`
	To     string `json:"to"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type RenamePlan struct {
	Error   string        `json:"error"`
	Ops     []RenameOpDTO `json:"ops"`
	Ready   int           `json:"ready"`
	Done    int           `json:"done"`
	CanUndo bool          `json:"canUndo"`
}

// RenameService renames the documents folder using values from the loaded
// table, always in two steps: Preview (no side effects) and Apply. The last
// Apply is remembered so it can be undone.
type RenameService struct {
	log      *slog.Logger
	campaign *CampaignService

	mu          sync.Mutex
	lastApplied []application.RenameOp
}

func NewRenameService(log *slog.Logger, campaign *CampaignService) *RenameService {
	return &RenameService{log: log, campaign: campaign}
}

func (r *RenameService) Preview(req RenameRequest) RenamePlan {
	ops, err := application.PlanRename(r.campaign.currentTable(), req.SourceColumn, req.Rule, req.Folder)
	if err != nil {
		return r.plan(nil, capitalize(err.Error()))
	}
	return r.plan(ops, "")
}

func (r *RenameService) Apply(req RenameRequest) RenamePlan {
	ops, err := application.PlanRename(r.campaign.currentTable(), req.SourceColumn, req.Rule, req.Folder)
	if err != nil {
		return r.plan(nil, capitalize(err.Error()))
	}
	result := application.ApplyRename(ops)

	var applied []application.RenameOp
	for _, op := range result {
		if op.Status == application.RenameDone {
			applied = append(applied, op)
		}
	}
	r.mu.Lock()
	r.lastApplied = applied
	r.mu.Unlock()
	r.log.Info("rename applied", "renamed", len(applied), "folder", req.Folder)
	return r.plan(result, "")
}

func (r *RenameService) Undo() RenamePlan {
	r.mu.Lock()
	applied := r.lastApplied
	r.lastApplied = nil
	r.mu.Unlock()

	undone := application.UndoRename(applied)
	r.log.Info("rename undone", "count", len(undone))
	return r.plan(undone, "")
}

func (r *RenameService) plan(ops []application.RenameOp, errMsg string) RenamePlan {
	p := RenamePlan{Error: errMsg, Ops: make([]RenameOpDTO, len(ops))}
	for i, op := range ops {
		p.Ops[i] = RenameOpDTO{Row: op.Row, From: base(op.From), To: base(op.To), Status: string(op.Status), Detail: op.Detail}
		switch op.Status {
		case application.RenameReady:
			p.Ready++
		case application.RenameDone:
			p.Done++
		}
	}
	r.mu.Lock()
	p.CanUndo = len(r.lastApplied) > 0
	r.mu.Unlock()
	return p
}

func base(p string) string {
	if p == "" {
		return ""
	}
	return filepath.Base(p)
}

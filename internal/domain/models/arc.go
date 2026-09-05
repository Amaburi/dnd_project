package models

import (
	"fmt"
	"strings"
	"time"
)

// StoryArc is a stretch of campaign with a shape: it builds, it peaks, it lands.
//
// StoryProgress used to hold MainQuestStage as a bare integer and CompletedArcs
// as bare strings, and nothing ever wrote either. Both are gone. A counter
// cannot say what the current act is about, and a string cannot say whether it
// is finished -- which is the one thing worth checking.
//
// An arc is made of plot threads. That is what makes its completion a fact
// rather than an opinion: an arc is over when the storylines it is made of are
// settled, and not before.
type StoryArc struct {
	ID         string `json:"id,omitempty" bson:"_id,omitempty"`
	ArcID      string `json:"arc_id" bson:"arc_id"`
	CampaignID string `json:"campaign_id" bson:"campaign_id"`

	Title   string `json:"title" bson:"title"`
	Premise string `json:"premise,omitempty" bson:"premise,omitempty"`

	// Order is where this arc sits in the campaign, 1-based. It replaces
	// MainQuestStage, which was a number with nothing attached to it.
	Order int `json:"order" bson:"order"`

	Status ArcStatus `json:"status" bson:"status"`
	Stage  ArcStage  `json:"stage" bson:"stage"`

	// ThreadIDs are the storylines this arc is made of.
	ThreadIDs []string `json:"thread_ids,omitempty" bson:"thread_ids,omitempty"`

	Resolution string `json:"resolution,omitempty" bson:"resolution,omitempty"`

	StartedSession   string `json:"started_session,omitempty" bson:"started_session,omitempty"`
	CompletedSession string `json:"completed_session,omitempty" bson:"completed_session,omitempty"`

	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" bson:"updated_at"`
	CompletedAt time.Time `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
}

// ArcStatus is where an arc stands in the campaign.
type ArcStatus string

const (
	ArcUpcoming  ArcStatus = "upcoming"
	ArcActive    ArcStatus = "active"
	ArcCompleted ArcStatus = "completed"
	ArcAbandoned ArcStatus = "abandoned"
)

// ArcStatuses lists every recognised status.
var ArcStatuses = []ArcStatus{ArcUpcoming, ArcActive, ArcCompleted, ArcAbandoned}

// Valid reports whether s is recognised. Empty means upcoming.
func (s ArcStatus) Valid() bool {
	if s == "" {
		return true
	}
	for _, known := range ArcStatuses {
		if s == known {
			return true
		}
	}
	return false
}

// ArcStage is where inside the arc the story currently is.
//
// It exists because pacing is the thing an AI DM is worst at unaided: without
// it every scene is pitched the same way, and a climax reads like a stroll.
type ArcStage string

const (
	ArcSetup      ArcStage = "setup"
	ArcRising     ArcStage = "rising"
	ArcClimax     ArcStage = "climax"
	ArcResolution ArcStage = "resolution"
)

// ArcStages lists the stages in order.
var ArcStages = []ArcStage{ArcSetup, ArcRising, ArcClimax, ArcResolution}

// Valid reports whether s is recognised. Empty means setup.
func (s ArcStage) Valid() bool {
	if s == "" {
		return true
	}
	for _, known := range ArcStages {
		if s == known {
			return true
		}
	}
	return false
}

// IsLive reports whether the arc is still in play.
func (a *StoryArc) IsLive() bool {
	return a.Status == ArcActive || a.Status == ArcUpcoming || a.Status == ""
}

// AdvanceStage moves the arc one step along its shape.
func (a *StoryArc) AdvanceStage() error {
	if !a.IsLive() {
		return Invalid("%q is %s and no longer advances", a.Title, a.Status)
	}

	current := a.Stage
	if current == "" {
		current = ArcSetup
	}
	for i, stage := range ArcStages {
		if stage != current {
			continue
		}
		if i+1 >= len(ArcStages) {
			return Invalid("%q is already at its resolution", a.Title)
		}
		a.Stage = ArcStages[i+1]
		a.UpdatedAt = time.Now().UTC()
		return nil
	}
	return Invalid("%q has an unknown stage %q", a.Title, a.Stage)
}

// Progress counts how many of the arc's threads are settled, and how many there
// are altogether.
//
// A thread the arc names but the campaign no longer has still counts towards
// the total. Silently shrinking the denominator would report an arc as nearly
// finished because one of its storylines went missing.
func (a *StoryArc) Progress(threads []*PlotThread) (settled, total int) {
	byID := make(map[string]*PlotThread, len(threads))
	for _, t := range threads {
		if t != nil {
			byID[t.ThreadID] = t
		}
	}

	total = len(a.ThreadIDs)
	for _, id := range a.ThreadIDs {
		if t, ok := byID[id]; ok && !t.IsLive() {
			settled++
		}
	}
	return settled, total
}

// CanComplete reports whether every storyline this arc is made of has been
// settled, and names the first that has not.
//
// Abandoned counts as settled: the party walked away, which is an answer.
func (a *StoryArc) CanComplete(threads []*PlotThread) (bool, string) {
	byID := make(map[string]*PlotThread, len(threads))
	for _, t := range threads {
		if t != nil {
			byID[t.ThreadID] = t
		}
	}

	var outstanding []string
	for _, id := range a.ThreadIDs {
		t, ok := byID[id]
		if !ok {
			// A thread the arc names but nothing has is not evidence that it
			// was finished, so it does not block. The count still shows it.
			continue
		}
		if t.IsLive() {
			label := t.Title
			if strings.TrimSpace(label) == "" {
				label = t.ThreadID
			}
			outstanding = append(outstanding, fmt.Sprintf("%s (%s)", label, t.ThreadID))
		}
	}

	if len(outstanding) > 0 {
		return false, fmt.Sprintf(
			"%q still has open storylines: %s. Resolve or abandon them first.",
			a.Title, strings.Join(outstanding, ", "))
	}
	return true, ""
}

// Complete finishes an arc that names no storylines.
//
// An arc that does name them must go through CompleteWith, which checks them.
// This deliberately refuses rather than skipping the check: an unchecked door
// standing open beside a checked one is a door someone will eventually walk
// through, and the check is the whole point of the type.
func (a *StoryArc) Complete(how, sessionID string) error {
	if len(a.ThreadIDs) > 0 {
		return Invalid(
			"%q is made of %d storylines; complete it with CompleteWith so they can be checked",
			a.Title, len(a.ThreadIDs))
	}
	return a.complete(how, sessionID)
}

// complete is the unchecked path, reachable only through the two doors above.
func (a *StoryArc) complete(how, sessionID string) error {
	if strings.TrimSpace(how) == "" {
		return Invalid("completing %q needs to say how it ended", a.Title)
	}
	if a.Status == ArcCompleted {
		return Invalid("%q is already complete", a.Title)
	}
	if a.Status == ArcAbandoned {
		return Invalid("%q was abandoned, not completed", a.Title)
	}

	now := time.Now().UTC()
	a.Status = ArcCompleted
	a.Stage = ArcResolution
	a.Resolution = how
	a.CompletedSession = sessionID
	a.CompletedAt, a.UpdatedAt = now, now
	return nil
}

// CompleteWith finishes the arc only if its storylines are settled.
//
// This is the rule that keeps the campaign honest: without it the DM believes
// an arc is done while the prompt still lists its threads as outstanding, and
// then narrates around a storyline it has quietly closed.
func (a *StoryArc) CompleteWith(how, sessionID string, threads []*PlotThread) error {
	if ok, reason := a.CanComplete(threads); !ok {
		return Invalid("%s", reason)
	}
	return a.complete(how, sessionID)
}

// Abandon records an arc the campaign walked away from.
func (a *StoryArc) Abandon(why string) error {
	if strings.TrimSpace(why) == "" {
		return Invalid("abandoning %q needs a reason", a.Title)
	}
	if a.Status == ArcCompleted {
		return Invalid("%q is complete, not abandoned", a.Title)
	}

	a.Status = ArcAbandoned
	a.Resolution = why
	a.UpdatedAt = time.Now().UTC()
	return nil
}

// WithThreads returns a copy of the arc naming a different set of threads.
func (a *StoryArc) WithThreads(threadIDs []string) *StoryArc {
	copied := *a
	copied.ThreadIDs = append([]string(nil), threadIDs...)
	return &copied
}

// Validate reports an arc that cannot mean anything.
func (a *StoryArc) Validate() error {
	var problems []string

	if strings.TrimSpace(a.Title) == "" {
		return Invalid("a story arc needs a title")
	}
	if strings.TrimSpace(a.CampaignID) == "" {
		problems = append(problems, "campaign_id is required")
	}
	if !a.Status.Valid() {
		problems = append(problems, fmt.Sprintf("unknown status %q", a.Status))
	}
	if !a.Stage.Valid() {
		problems = append(problems, fmt.Sprintf("unknown stage %q", a.Stage))
	}
	if a.Order < 0 {
		problems = append(problems, fmt.Sprintf("order %d is negative", a.Order))
	}
	// A finished arc with no resolution is a status change nobody can read.
	if (a.Status == ArcCompleted || a.Status == ArcAbandoned) && strings.TrimSpace(a.Resolution) == "" {
		problems = append(problems, fmt.Sprintf("a %s arc must say how it ended", a.Status))
	}

	if len(problems) > 0 {
		return Invalid("story arc %s: %s", a.Title, strings.Join(problems, "; "))
	}
	return nil
}

// Block describes where this arc stands, for the prompt.
func (a *StoryArc) Block(threads []*PlotThread) string {
	var b strings.Builder

	stage := a.Stage
	if stage == "" {
		stage = ArcSetup
	}
	fmt.Fprintf(&b, "Current arc: %s (%s)", a.Title, stage)
	if premise := strings.TrimSpace(a.Premise); premise != "" {
		fmt.Fprintf(&b, ". %s", premise)
	}

	if settled, total := a.Progress(threads); total > 0 {
		fmt.Fprintf(&b, " Storylines settled: %d of %d.", settled, total)
	}
	return b.String()
}

// ArcBlock renders the campaign's current position for the prompt.
//
// Never empty: a blank field invites the model to decide for itself what act
// the campaign is in, which is the pacing decision this type exists to make.
func ArcBlock(active *StoryArc, threads []*PlotThread) string {
	if active == nil || !active.IsLive() {
		return "No arc is currently running; the campaign is between chapters."
	}
	return active.Block(threads)
}

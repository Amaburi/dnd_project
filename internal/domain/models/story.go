package models

import (
	"fmt"
	"strings"
	"time"
)

// Plot threads and consequences: the DM's outstanding work.
//
// StoryProgress used to hold ActivePlotThreads as bare strings and StoryEvent
// holds free-text Consequences, and nothing ever wrote either. A bare string
// cannot be resolved, cannot age, and cannot be linked to what caused it -- so
// a thread the DM opened stayed open for ever and a choice the party made never
// came back. These are the tracked versions.

// ThreadStatus is where a storyline stands.
type ThreadStatus string

const (
	// ThreadOpen is live and being pursued.
	ThreadOpen ThreadStatus = "open"

	// ThreadDormant is live but gone quiet. It is deliberately distinct from
	// abandoned: the party has not walked away, the story just moved elsewhere.
	ThreadDormant ThreadStatus = "dormant"

	// ThreadResolved is finished. It is kept, not deleted: a resolved thread is
	// the record of what happened.
	ThreadResolved ThreadStatus = "resolved"

	// ThreadAbandoned is one the party walked away from.
	ThreadAbandoned ThreadStatus = "abandoned"
)

// ThreadStatuses lists every recognised status.
var ThreadStatuses = []ThreadStatus{ThreadOpen, ThreadDormant, ThreadResolved, ThreadAbandoned}

// Valid reports whether s is recognised. The empty status is open, so a thread
// created without one is live rather than in limbo.
func (s ThreadStatus) Valid() bool {
	if s == "" {
		return true
	}
	for _, known := range ThreadStatuses {
		if s == known {
			return true
		}
	}
	return false
}

// ThreadUrgency is how much the thread is pressing on the party right now.
type ThreadUrgency string

const (
	ThreadBackground ThreadUrgency = "background"
	ThreadActive     ThreadUrgency = "active"
	ThreadPressing   ThreadUrgency = "pressing"
)

// ThreadUrgencies lists every recognised urgency.
var ThreadUrgencies = []ThreadUrgency{ThreadBackground, ThreadActive, ThreadPressing}

// Valid reports whether u is recognised. Empty means active.
func (u ThreadUrgency) Valid() bool {
	if u == "" {
		return true
	}
	for _, known := range ThreadUrgencies {
		if u == known {
			return true
		}
	}
	return false
}

// MaxThreadBeats bounds a thread's history.
//
// Unlike an NPC's memory, the oldest beats are the ones to drop: a thread's
// current state is what the DM needs, where a grudge is defined by its first
// betrayal.
const MaxThreadBeats = 8

// ThreadBeat is one development on a storyline.
type ThreadBeat struct {
	Summary   string    `json:"summary" bson:"summary"`
	SessionID string    `json:"session_id,omitempty" bson:"session_id,omitempty"`
	At        time.Time `json:"at" bson:"at"`
}

// PlotThread is a storyline the DM has opened and must not drop.
type PlotThread struct {
	ID         string `json:"id,omitempty" bson:"_id,omitempty"`
	ThreadID   string `json:"thread_id" bson:"thread_id"`
	CampaignID string `json:"campaign_id" bson:"campaign_id"`

	Title   string `json:"title" bson:"title"`
	Summary string `json:"summary,omitempty" bson:"summary,omitempty"`

	Status  ThreadStatus  `json:"status" bson:"status"`
	Urgency ThreadUrgency `json:"urgency,omitempty" bson:"urgency,omitempty"`

	// Involves names the people and places this touches, so the DM can connect
	// a thread to the person standing in front of the party.
	Involves []string `json:"involves,omitempty" bson:"involves,omitempty"`

	Beats []ThreadBeat `json:"beats,omitempty" bson:"beats,omitempty"`

	// Resolution is how it ended, for a resolved or abandoned thread.
	Resolution string `json:"resolution,omitempty" bson:"resolution,omitempty"`

	OpenedSession   string `json:"opened_session,omitempty" bson:"opened_session,omitempty"`
	ResolvedSession string `json:"resolved_session,omitempty" bson:"resolved_session,omitempty"`

	OpenedAt   time.Time `json:"opened_at" bson:"opened_at"`
	UpdatedAt  time.Time `json:"updated_at" bson:"updated_at"`
	ResolvedAt time.Time `json:"resolved_at,omitempty" bson:"resolved_at,omitempty"`
}

// IsLive reports whether the thread is still outstanding work.
//
// Dormant counts: it has gone quiet, not away.
func (p *PlotThread) IsLive() bool {
	return p.Status == ThreadOpen || p.Status == ThreadDormant || p.Status == ""
}

// Advance records a development.
func (p *PlotThread) Advance(beat ThreadBeat) error {
	if strings.TrimSpace(beat.Summary) == "" {
		return Invalid("a plot beat needs a summary")
	}
	if !p.IsLive() {
		return Invalid("%q is %s and cannot gain new developments", p.Title, p.Status)
	}

	if beat.At.IsZero() {
		beat.At = time.Now().UTC()
	}
	p.Beats = append(p.Beats, beat)
	if len(p.Beats) > MaxThreadBeats {
		p.Beats = p.Beats[len(p.Beats)-MaxThreadBeats:]
	}
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// Resolve finishes the thread. It is final: resolving twice would overwrite the
// answer with a second one.
func (p *PlotThread) Resolve(how, sessionID string) error {
	if strings.TrimSpace(how) == "" {
		return Invalid("resolving %q needs to say how it ended", p.Title)
	}
	if p.Status == ThreadResolved {
		return Invalid("%q is already resolved", p.Title)
	}

	now := time.Now().UTC()
	p.Status = ThreadResolved
	p.Resolution = how
	p.ResolvedSession = sessionID
	p.ResolvedAt, p.UpdatedAt = now, now
	return nil
}

// Abandon marks a thread the party walked away from.
func (p *PlotThread) Abandon(why string) error {
	if strings.TrimSpace(why) == "" {
		return Invalid("abandoning %q needs a reason", p.Title)
	}
	if p.Status == ThreadResolved {
		return Invalid("%q is resolved, not abandoned", p.Title)
	}

	p.Status = ThreadAbandoned
	p.Resolution = why
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// SetDormant quiets a thread without closing it.
func (p *PlotThread) SetDormant() error {
	if !p.IsLive() {
		return Invalid("%q is %s and cannot go dormant", p.Title, p.Status)
	}
	p.Status = ThreadDormant
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// Reopen brings a thread back.
//
// An abandoned thread can be picked up again -- players change their minds --
// but a resolved one cannot: it is finished, and reopening it would mean the
// recorded ending never happened.
func (p *PlotThread) Reopen() error {
	if p.Status == ThreadResolved {
		return Invalid("%q is resolved; its ending already happened", p.Title)
	}
	p.Status = ThreadOpen
	p.Resolution = ""
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// Validate reports a thread that cannot mean anything.
func (p *PlotThread) Validate() error {
	var problems []string

	if strings.TrimSpace(p.Title) == "" {
		return Invalid("a plot thread needs a title")
	}
	if strings.TrimSpace(p.CampaignID) == "" {
		problems = append(problems, "campaign_id is required")
	}
	if !p.Status.Valid() {
		problems = append(problems, fmt.Sprintf("unknown status %q", p.Status))
	}
	if !p.Urgency.Valid() {
		problems = append(problems, fmt.Sprintf("unknown urgency %q", p.Urgency))
	}
	for i, beat := range p.Beats {
		if strings.TrimSpace(beat.Summary) == "" {
			problems = append(problems, fmt.Sprintf("beat %d has no summary", i+1))
		}
	}
	if len(problems) > 0 {
		return Invalid("plot thread %s: %s", p.Title, strings.Join(problems, "; "))
	}
	return nil
}

// ConsequenceStatus is whether a consequence has landed.
type ConsequenceStatus string

const (
	// ConsequencePending has not happened yet, which is the whole point.
	ConsequencePending ConsequenceStatus = "pending"

	ConsequenceRealised ConsequenceStatus = "realised"
	ConsequenceAverted  ConsequenceStatus = "averted"
	ConsequenceExpired  ConsequenceStatus = "expired"
)

// ConsequenceStatuses lists every recognised status.
var ConsequenceStatuses = []ConsequenceStatus{
	ConsequencePending, ConsequenceRealised, ConsequenceAverted, ConsequenceExpired,
}

// Valid reports whether s is recognised. Empty means pending.
func (s ConsequenceStatus) Valid() bool {
	if s == "" {
		return true
	}
	for _, known := range ConsequenceStatuses {
		if s == known {
			return true
		}
	}
	return false
}

// ConsequenceSeverity is how much it will matter when it lands.
type ConsequenceSeverity string

const (
	SeverityMinor    ConsequenceSeverity = "minor"
	SeverityModerate ConsequenceSeverity = "moderate"
	SeverityMajor    ConsequenceSeverity = "major"
)

// ConsequenceSeverities lists every recognised severity.
var ConsequenceSeverities = []ConsequenceSeverity{SeverityMinor, SeverityModerate, SeverityMajor}

// Valid reports whether s is recognised. Empty means moderate.
func (s ConsequenceSeverity) Valid() bool {
	if s == "" {
		return true
	}
	for _, known := range ConsequenceSeverities {
		if s == known {
			return true
		}
	}
	return false
}

// Consequence is something the party set in motion that has not landed yet.
//
// This is what makes a choice a choice: sparing the goblin chief means nothing
// unless something remembers, and remembers that it has not come back around.
type Consequence struct {
	ID            string `json:"id,omitempty" bson:"_id,omitempty"`
	ConsequenceID string `json:"consequence_id" bson:"consequence_id"`
	CampaignID    string `json:"campaign_id" bson:"campaign_id"`

	// ThreadID optionally ties this to a storyline.
	ThreadID string `json:"thread_id,omitempty" bson:"thread_id,omitempty"`

	Cause    string `json:"cause" bson:"cause"`
	Expected string `json:"expected" bson:"expected"`

	Severity ConsequenceSeverity `json:"severity,omitempty" bson:"severity,omitempty"`
	Status   ConsequenceStatus   `json:"status" bson:"status"`

	// Outcome is what actually happened, once it stopped being pending.
	Outcome string `json:"outcome,omitempty" bson:"outcome,omitempty"`

	SourceEventID   string `json:"source_event_id,omitempty" bson:"source_event_id,omitempty"`
	SessionID       string `json:"session_id,omitempty" bson:"session_id,omitempty"`
	ResolvedSession string `json:"resolved_session,omitempty" bson:"resolved_session,omitempty"`

	CreatedAt  time.Time `json:"created_at" bson:"created_at"`
	ResolvedAt time.Time `json:"resolved_at,omitempty" bson:"resolved_at,omitempty"`
}

// IsPending reports whether this is still owed to the party.
func (c *Consequence) IsPending() bool {
	return c.Status == ConsequencePending || c.Status == ""
}

// settle is the shared path for a consequence ceasing to be pending.
func (c *Consequence) settle(status ConsequenceStatus, outcome, sessionID string) error {
	if strings.TrimSpace(outcome) == "" {
		return Invalid("a consequence needs to say what happened")
	}
	if !c.IsPending() {
		return Invalid("this consequence is already %s", c.Status)
	}

	c.Status = status
	c.Outcome = outcome
	c.ResolvedSession = sessionID
	c.ResolvedAt = time.Now().UTC()
	return nil
}

// Realise records the consequence landing. It can only land once: a second
// telling would overwrite the first.
func (c *Consequence) Realise(what, sessionID string) error {
	return c.settle(ConsequenceRealised, what, sessionID)
}

// Avert records the party heading it off.
func (c *Consequence) Avert(how, sessionID string) error {
	return c.settle(ConsequenceAverted, how, sessionID)
}

// Expire records a consequence that stopped mattering.
func (c *Consequence) Expire(why string) error {
	return c.settle(ConsequenceExpired, why, "")
}

// Validate reports a consequence that cannot mean anything.
func (c *Consequence) Validate() error {
	var problems []string

	if strings.TrimSpace(c.Cause) == "" {
		problems = append(problems, "a consequence needs a cause")
	}
	if strings.TrimSpace(c.Expected) == "" {
		problems = append(problems, "a consequence needs to say what is expected to follow")
	}
	if strings.TrimSpace(c.CampaignID) == "" {
		problems = append(problems, "campaign_id is required")
	}
	if !c.Status.Valid() {
		problems = append(problems, fmt.Sprintf("unknown status %q", c.Status))
	}
	if !c.Severity.Valid() {
		problems = append(problems, fmt.Sprintf("unknown severity %q", c.Severity))
	}
	// A settled consequence with no outcome is a status change nobody can read.
	if !c.IsPending() && strings.TrimSpace(c.Outcome) == "" {
		problems = append(problems, fmt.Sprintf("a %s consequence must say what happened", c.Status))
	}

	if len(problems) > 0 {
		return Invalid("consequence: %s", strings.Join(problems, "; "))
	}
	return nil
}

// StoryBlock renders the DM's outstanding work for a prompt.
//
// Only what is outstanding: a resolved thread and a landed consequence are
// history, and history belongs in the rolling summary rather than in the list
// of things still owed. Sending finished business would spend the budget on
// what the DM has no further obligation to.
func StoryBlock(threads []*PlotThread, consequences []*Consequence) string {
	var b strings.Builder

	var live []*PlotThread
	for _, t := range threads {
		if t != nil && t.IsLive() {
			live = append(live, t)
		}
	}
	var pending []*Consequence
	for _, c := range consequences {
		if c != nil && c.IsPending() {
			pending = append(pending, c)
		}
	}

	if len(live) == 0 && len(pending) == 0 {
		return "There are no open plot threads and nothing the party has set in motion is still pending."
	}

	if len(live) > 0 {
		b.WriteString("Open plot threads:")
		for _, t := range live {
			fmt.Fprintf(&b, "\n- %s", t.Title)
			if urgency := t.Urgency; urgency != "" && urgency != ThreadActive {
				fmt.Fprintf(&b, " (%s)", urgency)
			}
			if summary := strings.TrimSpace(t.Summary); summary != "" {
				fmt.Fprintf(&b, ": %s", summary)
			}
			if len(t.Beats) > 0 {
				fmt.Fprintf(&b, " Most recently: %s", strings.TrimSpace(t.Beats[len(t.Beats)-1].Summary))
			}
		}
	}

	if len(pending) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("Set in motion by the party, not yet come back around:")
		for _, c := range pending {
			fmt.Fprintf(&b, "\n- %s -> %s", strings.TrimSpace(c.Cause), strings.TrimSpace(c.Expected))
			if c.Severity == SeverityMajor {
				b.WriteString(" (major)")
			}
		}
	}
	return b.String()
}

//go:build integration

// Integration tests run against a real MongoDB. They are behind a build tag
// rather than a t.Skip so the default suite stays honest: `go test ./...`
// never silently passes over them, and a run that includes the tag either
// exercises a database or fails saying why.
//
//	MONGODB_TEST_URI=mongodb://localhost:27017 go test -tags=integration ./...
package mongodb

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dnd-campaign/manager/internal/domain/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// testDB connects to the configured MongoDB and returns a client pointed at a
// scratch database, dropped when the test ends.
//
// Every test gets its own database rather than sharing one and cleaning up
// between: a leaked document then cannot make an unrelated test fail, which is
// the usual way an integration suite becomes untrustworthy.
func testDB(t *testing.T) *Client {
	t.Helper()

	uri := os.Getenv("MONGODB_TEST_URI")
	if uri == "" {
		t.Fatal("MONGODB_TEST_URI is not set; integration tests need a database " +
			"(e.g. MONGODB_TEST_URI=mongodb://localhost:27017)")
	}

	name := "dnd_test_" + primitive.NewObjectID().Hex()
	client, err := NewClient(Config{
		URI: uri, Database: name,
		MaxPoolSize: 10, MinPoolSize: 1, ConnectTimeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("connecting to %s: %v", uri, err)
	}

	ctx := context.Background()
	if err := client.InitializeCollections(ctx); err != nil {
		t.Fatalf("initialising collections: %v", err)
	}
	if err := client.CreateIndexes(ctx); err != nil {
		t.Fatalf("creating indexes: %v", err)
	}

	t.Cleanup(func() {
		dropCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = client.Database().Drop(dropCtx)
		_ = client.Close(dropCtx)
	})

	return client
}

func ctx() context.Context { return context.Background() }

func newCampaign(t *testing.T, repo *CampaignRepository) *models.Campaign {
	t.Helper()
	campaign := &models.Campaign{Title: "The Sunken Crypt", CreatedBy: "dm1"}
	if err := repo.CreateCampaign(ctx(), campaign); err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	return campaign
}

// --- campaigns --------------------------------------------------------------

func TestCampaignRoundTrip(t *testing.T) {
	repo := NewCampaignRepository(testDB(t))

	campaign := newCampaign(t, repo)
	if campaign.ID.IsZero() {
		t.Fatal("the inserted id was not written back")
	}
	if campaign.CampaignID == "" {
		t.Fatal("no business id was generated")
	}
	if campaign.CreatedAt.IsZero() {
		t.Fatal("created_at was not set")
	}

	found, err := repo.GetCampaignByID(ctx(), campaign.ID)
	if err != nil || found == nil {
		t.Fatalf("GetCampaignByID = %v, %v", found, err)
	}
	if found.Title != "The Sunken Crypt" {
		t.Errorf("title = %q", found.Title)
	}

	// A missing document is (nil, nil), not an error.
	missing, err := repo.GetCampaignByID(ctx(), primitive.NewObjectID())
	if err != nil || missing != nil {
		t.Errorf("a missing campaign returned %v, %v; want nil, nil", missing, err)
	}
}

// The bug this guards: $set-ing the whole struct blanked campaign_id, which is
// uniquely indexed, so the *second* such update died on a duplicate key.
func TestCampaignUpdateKeepsImmutableFields(t *testing.T) {
	repo := NewCampaignRepository(testDB(t))

	first := newCampaign(t, repo)
	second := &models.Campaign{Title: "Another", CreatedBy: "dm1"}
	if err := repo.CreateCampaign(ctx(), second); err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}

	// Update both the way a PUT does: a struct decoded from a body, carrying
	// only the fields the client sent.
	for _, id := range []primitive.ObjectID{first.ID, second.ID} {
		update := &models.Campaign{ID: id, Title: "Renamed"}
		if err := repo.UpdateCampaign(ctx(), update); err != nil {
			t.Fatalf("UpdateCampaign: %v", err)
		}
	}

	after, _ := repo.GetCampaignByID(ctx(), first.ID)
	if after.CampaignID != first.CampaignID {
		t.Errorf("campaign_id changed from %q to %q", first.CampaignID, after.CampaignID)
	}
	if after.CreatedBy != "dm1" {
		t.Errorf("created_by was blanked to %q", after.CreatedBy)
	}
	if after.CreatedAt.IsZero() {
		t.Error("created_at was zeroed by the update")
	}
	if after.Title != "Renamed" {
		t.Errorf("the mutable field did not change: %q", after.Title)
	}
}

func TestCampaignListAndDelete(t *testing.T) {
	repo := NewCampaignRepository(testDB(t))

	empty, err := repo.GetCampaignsByUser(ctx(), "nobody")
	if err != nil {
		t.Fatalf("GetCampaignsByUser: %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Errorf("an empty result = %v; want [] rather than null", empty)
	}

	campaign := newCampaign(t, repo)
	found, _ := repo.GetCampaignsByUser(ctx(), "dm1")
	if len(found) != 1 {
		t.Fatalf("got %d campaigns, want 1", len(found))
	}

	if err := repo.DeleteCampaign(ctx(), campaign.ID); err != nil {
		t.Fatalf("DeleteCampaign: %v", err)
	}
	// Deleting twice is a not-found, not a silent success.
	err = repo.DeleteCampaign(ctx(), campaign.ID)
	if err == nil {
		t.Error("deleting a missing campaign should fail")
	}
}

// --- characters -------------------------------------------------------------

func validCharacter(campaignID string) *models.Character {
	return &models.Character{
		CampaignID: campaignID, Name: "Thistle", Type: models.CharacterPlayer,
		BasicInfo: models.BasicInfo{
			Race: models.RaceHalfling, Subrace: "lightfoot",
			Background: models.BackgroundCriminal,
			Classes:    []models.ClassLevel{{Class: models.ClassRogue, Subclass: "thief", Level: 3}},
		},
		AbilityScores: models.AbilityScores{
			Strength: 10, Dexterity: 17, Constitution: 14,
			Intelligence: 12, Wisdom: 13, Charisma: 11,
		},
		Skills: models.SkillProficiencies{
			models.SkillDeception:     models.ProficiencyProficient,
			models.SkillStealth:       models.ProficiencyExpertise,
			models.SkillAcrobatics:    models.ProficiencyProficient,
			models.SkillInvestigation: models.ProficiencyProficient,
		},
	}
}

func TestCharacterRoundTripAndScoping(t *testing.T) {
	db := testDB(t)
	campaigns := NewCampaignRepository(db)
	characters := NewCharacterRepository(db)

	one := newCampaign(t, campaigns)
	other := &models.Campaign{Title: "Elsewhere", CreatedBy: "dm1"}
	if err := campaigns.CreateCampaign(ctx(), other); err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}

	character := validCharacter(one.CampaignID)
	if err := characters.CreateCharacter(ctx(), character); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	found, err := characters.GetCharacterInCampaign(ctx(), one.CampaignID, character.ID)
	if err != nil || found == nil {
		t.Fatalf("GetCharacterInCampaign = %v, %v", found, err)
	}

	// The scoping fix: the same character is invisible through another
	// campaign's id, which is what stopped cross-campaign access.
	leaked, err := characters.GetCharacterInCampaign(ctx(), other.CampaignID, character.ID)
	if err != nil {
		t.Fatalf("GetCharacterInCampaign: %v", err)
	}
	if leaked != nil {
		t.Error("a character was reachable through an unrelated campaign")
	}
}

// CreateCharacter validates the sheet; UpdateCharacter deliberately does not.
func TestCharacterCreateValidatesTheSheet(t *testing.T) {
	db := testDB(t)
	campaign := newCampaign(t, NewCampaignRepository(db))
	characters := NewCharacterRepository(db)

	illegal := validCharacter(campaign.CampaignID)
	illegal.BasicInfo.Classes[0].Subclass = "champion" // a fighter archetype

	if err := characters.CreateCharacter(ctx(), illegal); err == nil {
		t.Fatal("an illegal sheet was accepted")
	}

	legal := validCharacter(campaign.CampaignID)
	if err := characters.CreateCharacter(ctx(), legal); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	legal.BasicInfo.Classes[0].Subclass = "champion"
	if err := characters.UpdateCharacter(ctx(), campaign.CampaignID, legal); err != nil {
		t.Errorf("an update was blocked by sheet validation: %v", err)
	}
}

func TestCharacterConditionsAndHitPoints(t *testing.T) {
	db := testDB(t)
	campaign := newCampaign(t, NewCampaignRepository(db))
	characters := NewCharacterRepository(db)

	character := validCharacter(campaign.CampaignID)
	if err := characters.CreateCharacter(ctx(), character); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	if err := characters.AddCondition(ctx(), character.ID, models.ConditionProne); err != nil {
		t.Fatalf("AddCondition: %v", err)
	}
	// Applying twice must not duplicate.
	if err := characters.AddCondition(ctx(), character.ID, models.ConditionProne); err != nil {
		t.Fatalf("AddCondition twice: %v", err)
	}
	if err := characters.AddCondition(ctx(), character.ID, "inspired"); err == nil {
		t.Error("an invented condition was stored")
	}

	if err := characters.UpdateCharacterHP(ctx(), character.ID, 4, 24, 3); err != nil {
		t.Fatalf("UpdateCharacterHP: %v", err)
	}
	if err := characters.SetExhaustion(ctx(), character.ID, 2); err != nil {
		t.Fatalf("SetExhaustion: %v", err)
	}
	if err := characters.SetExhaustion(ctx(), character.ID, 9); err == nil {
		t.Error("an exhaustion level above 6 was accepted")
	}

	after, _ := characters.GetCharacterInCampaign(ctx(), campaign.CampaignID, character.ID)
	if len(after.Conditions) != 1 || after.Conditions[0] != models.ConditionProne {
		t.Errorf("conditions = %v, want one prone", after.Conditions)
	}
	if after.CombatStats.HitPoints.Current != 4 || after.CombatStats.HitPoints.Temporary != 3 {
		t.Errorf("hit points = %+v", after.CombatStats.HitPoints)
	}
	if after.Exhaustion != 2 {
		t.Errorf("exhaustion = %d, want 2", after.Exhaustion)
	}
}

// The search fix: a regex metacharacter is quoted rather than interpreted.
func TestCharacterSearchEscapesTheQuery(t *testing.T) {
	db := testDB(t)
	campaign := newCampaign(t, NewCampaignRepository(db))
	characters := NewCharacterRepository(db)

	for _, name := range []string{"Thistle", "Bram", "A.B"} {
		c := validCharacter(campaign.CampaignID)
		c.Name = name
		if err := characters.CreateCharacter(ctx(), c); err != nil {
			t.Fatalf("CreateCharacter %s: %v", name, err)
		}
	}

	found, err := characters.SearchCharacters(ctx(), campaign.CampaignID, "this")
	if err != nil {
		t.Fatalf("SearchCharacters: %v", err)
	}
	if len(found) != 1 || found[0].Name != "Thistle" {
		t.Errorf("search for 'this' = %v", names(found))
	}

	// "A.B" must match literally; an unescaped dot would also match "AxB".
	dotted, _ := characters.SearchCharacters(ctx(), campaign.CampaignID, "A.B")
	if len(dotted) != 1 {
		t.Errorf("search for 'A.B' = %v", names(dotted))
	}
	// A lone dot is a literal too, so it matches only the name containing one.
	literal, _ := characters.SearchCharacters(ctx(), campaign.CampaignID, ".")
	if len(literal) != 1 {
		t.Errorf("search for '.' matched %v; the query should be quoted", names(literal))
	}

	all, _ := characters.SearchCharacters(ctx(), campaign.CampaignID, "")
	if len(all) != 3 {
		t.Errorf("an empty query returned %d, want every character", len(all))
	}
}

func names(characters []*models.Character) []string {
	out := make([]string, 0, len(characters))
	for _, c := range characters {
		out = append(out, c.Name)
	}
	return out
}

// --- monsters ---------------------------------------------------------------

func TestMonsterSeedAndQueries(t *testing.T) {
	db := testDB(t)
	campaign := newCampaign(t, NewCampaignRepository(db))
	monsters := NewMonsterRepository(db)

	seeded, err := monsters.SeedSRDMonsters(ctx(), campaign.CampaignID)
	if err != nil {
		t.Fatalf("SeedSRDMonsters: %v", err)
	}
	if seeded != len(models.SRDMonsters()) {
		t.Errorf("seeded %d, want %d", seeded, len(models.SRDMonsters()))
	}

	// Seeding twice adds nothing, so the endpoint is safe to call again.
	again, err := monsters.SeedSRDMonsters(ctx(), campaign.CampaignID)
	if err != nil {
		t.Fatalf("second SeedSRDMonsters: %v", err)
	}
	if again != 0 {
		t.Errorf("re-seeding added %d monsters", again)
	}

	all, _ := monsters.GetMonstersByCampaign(ctx(), campaign.CampaignID)
	if len(all) != seeded {
		t.Errorf("got %d monsters, want %d", len(all), seeded)
	}

	low, err := monsters.GetMonstersByChallengeRating(ctx(), campaign.CampaignID, 0, 0.25)
	if err != nil {
		t.Fatalf("GetMonstersByChallengeRating: %v", err)
	}
	for _, m := range low {
		if m.ChallengeRating > 0.25 {
			t.Errorf("%s is CR %v, outside the band", m.Name, m.ChallengeRating)
		}
	}

	troll, _ := monsters.SearchMonsters(ctx(), campaign.CampaignID, "troll")
	if len(troll) != 1 {
		t.Fatalf("search for troll returned %d", len(troll))
	}

	// Damage that is not persisted is damage that did not happen.
	hurt := models.HitPoints{Current: 12, Maximum: troll[0].HitPoints.Maximum}
	if err := monsters.UpdateHitPoints(ctx(), campaign.CampaignID, troll[0].MonsterID, hurt); err != nil {
		t.Fatalf("UpdateHitPoints: %v", err)
	}
	after, _ := monsters.SearchMonsters(ctx(), campaign.CampaignID, "troll")
	if after[0].HitPoints.Current != 12 {
		t.Errorf("hit points = %d, want the persisted 12", after[0].HitPoints.Current)
	}
}

func TestMonsterCreateValidatesTheStatblock(t *testing.T) {
	db := testDB(t)
	campaign := newCampaign(t, NewCampaignRepository(db))
	monsters := NewMonsterRepository(db)

	broken := &models.Monster{
		CampaignID: campaign.CampaignID, Name: "Impossible",
		Size: models.SizeMedium, ArmorClass: 12,
		HitPoints: models.HitPoints{Current: 99, Maximum: 99}, HitDice: "2d6",
		ChallengeRating: 1,
	}
	// 2d6 averages 7, not 99.
	if err := monsters.CreateMonster(ctx(), broken); err == nil {
		t.Error("a statblock whose hit points contradict its dice was accepted")
	}
}

// --- sessions and the event log ---------------------------------------------

func TestSessionLifecycle(t *testing.T) {
	db := testDB(t)
	campaign := newCampaign(t, NewCampaignRepository(db))
	sessions := NewSessionRepository(db)

	first := &models.Session{CampaignID: campaign.CampaignID, Title: "Into the crypt"}
	if err := sessions.CreateSession(ctx(), first); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if first.SessionNumber != 1 {
		t.Errorf("session number = %d, want 1", first.SessionNumber)
	}

	second := &models.Session{CampaignID: campaign.CampaignID, Title: "Deeper"}
	if err := sessions.CreateSession(ctx(), second); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if second.SessionNumber != 2 {
		t.Errorf("session number = %d, want 2 (assigned in sequence)", second.SessionNumber)
	}

	if active, _ := sessions.GetActiveSession(ctx(), campaign.CampaignID); active != nil {
		t.Error("a session is in progress before any was started")
	}

	if _, err := sessions.StartSession(ctx(), campaign.CampaignID, first.ID); err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	active, _ := sessions.GetActiveSession(ctx(), campaign.CampaignID)
	if active == nil || active.SessionNumber != 1 {
		t.Fatalf("active session = %v, want session 1", active)
	}
	if active.Date.ActualStart == nil {
		t.Error("the start time was not recorded")
	}

	// Starting another closes the first: two live sessions would make "which
	// one does this event belong to" unanswerable.
	if _, err := sessions.StartSession(ctx(), campaign.CampaignID, second.ID); err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	active, _ = sessions.GetActiveSession(ctx(), campaign.CampaignID)
	if active == nil || active.SessionNumber != 2 {
		t.Fatalf("active session = %v, want session 2", active)
	}

	closed, _ := sessions.GetSessionInCampaign(ctx(), campaign.CampaignID, first.ID)
	if closed.Status != models.SessionStatusCompleted {
		t.Errorf("the first session is %q, want completed", closed.Status)
	}

	if _, err := sessions.EndSession(ctx(), campaign.CampaignID, second.ID); err != nil {
		t.Fatalf("EndSession: %v", err)
	}
	if _, err := sessions.EndSession(ctx(), campaign.CampaignID, second.ID); err == nil {
		t.Error("ending a finished session should fail")
	}
}

// The log is the campaign's memory, and appending is where the pieces meet.
func TestStoryEventAppendFoldsIntoTheSession(t *testing.T) {
	db := testDB(t)
	campaign := newCampaign(t, NewCampaignRepository(db))
	sessions := NewSessionRepository(db)
	events := NewStoryEventRepository(db, sessions)

	session := &models.Session{CampaignID: campaign.CampaignID, Title: "Into the crypt"}
	if err := sessions.CreateSession(ctx(), session); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, err := sessions.StartSession(ctx(), campaign.CampaignID, session.ID); err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	for i, natural := range []int{20, 1, 12} {
		event := &models.StoryEvent{
			CampaignID: campaign.CampaignID, SessionID: session.SessionID,
			EventType: EventDiceRoll,
			Narrative: models.NarrativeInfo{
				AIGeneratedText: "line " + string(rune('a'+i)),
				DiceResults:     &models.DiceResults{Roll: models.D20Result{Natural: natural, Total: natural}},
			},
			AIContext: models.AIContextInfo{PromptTokens: 100, CompletionTokens: 50},
			Metadata:  models.EventMetadata{CostUSD: 0.001},
		}
		if err := events.AppendEvent(ctx(), event); err != nil {
			t.Fatalf("AppendEvent: %v", err)
		}
		if event.SequenceNumber != i+1 {
			t.Errorf("sequence = %d, want %d", event.SequenceNumber, i+1)
		}
	}

	// Dice history accrues on the session without a separate bookkeeping path.
	after, _ := sessions.GetSessionInCampaign(ctx(), campaign.CampaignID, session.ID)
	if after.DiceRollsSummary.TotalRolls != 3 {
		t.Errorf("total rolls = %d, want 3", after.DiceRollsSummary.TotalRolls)
	}
	if after.DiceRollsSummary.Natural20s != 1 || after.DiceRollsSummary.Natural1s != 1 {
		t.Errorf("nat20/nat1 = %d/%d, want 1/1",
			after.DiceRollsSummary.Natural20s, after.DiceRollsSummary.Natural1s)
	}
	if after.AIInteractions.TotalPrompts != 3 || after.AIInteractions.TotalTokensUsed != 450 {
		t.Errorf("ai usage = %+v", after.AIInteractions)
	}

	ordered, err := events.GetEventsBySession(ctx(), campaign.CampaignID, session.SessionID)
	if err != nil {
		t.Fatalf("GetEventsBySession: %v", err)
	}
	for i, event := range ordered {
		if event.SequenceNumber != i+1 {
			t.Errorf("event %d has sequence %d", i, event.SequenceNumber)
		}
	}

	// Recent events come back oldest first, because a prompt reads better in
	// the order things happened.
	recent, err := events.GetRecentEvents(ctx(), campaign.CampaignID, 2)
	if err != nil {
		t.Fatalf("GetRecentEvents: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("got %d recent events, want 2", len(recent))
	}
	if recent[0].SequenceNumber >= recent[1].SequenceNumber {
		t.Errorf("recent events are not oldest-first: %d then %d",
			recent[0].SequenceNumber, recent[1].SequenceNumber)
	}
}

// The unique index on (campaign, session, sequence) is what makes AppendEvent
// safe to call concurrently: a lost race retries into the next slot.
func TestStoryEventAppendIsConcurrencySafe(t *testing.T) {
	db := testDB(t)
	campaign := newCampaign(t, NewCampaignRepository(db))
	sessions := NewSessionRepository(db)
	events := NewStoryEventRepository(db, sessions)

	session := &models.Session{CampaignID: campaign.CampaignID, Title: "Busy"}
	if err := sessions.CreateSession(ctx(), session); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	const writers = 5
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		go func() {
			errs <- events.AppendEvent(ctx(), &models.StoryEvent{
				CampaignID: campaign.CampaignID, SessionID: session.SessionID,
				Narrative: models.NarrativeInfo{AIGeneratedText: "concurrent"},
			})
		}()
	}
	for i := 0; i < writers; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent AppendEvent: %v", err)
		}
	}

	logged, _ := events.GetEventsBySession(ctx(), campaign.CampaignID, session.SessionID)
	if len(logged) != writers {
		t.Fatalf("got %d events, want %d", len(logged), writers)
	}
	seen := map[int]bool{}
	for _, event := range logged {
		if seen[event.SequenceNumber] {
			t.Errorf("sequence %d was used twice", event.SequenceNumber)
		}
		seen[event.SequenceNumber] = true
	}
}

// --- encounters -------------------------------------------------------------

func TestEncounterPersistsItsLogs(t *testing.T) {
	db := testDB(t)
	campaign := newCampaign(t, NewCampaignRepository(db))
	encounters := NewEncounterRepository(db)

	encounter := &models.CombatEncounter{
		CampaignID: campaign.CampaignID, EncounterName: "Cellar ambush",
	}
	if err := encounters.CreateEncounter(ctx(), encounter); err != nil {
		t.Fatalf("CreateEncounter: %v", err)
	}
	if encounter.CombatState.Phase != models.PhaseNotStarted {
		t.Errorf("phase = %q, want not_started", encounter.CombatState.Phase)
	}

	active, err := encounters.GetActiveEncounter(ctx(), campaign.CampaignID)
	if err != nil || active == nil {
		t.Fatalf("GetActiveEncounter = %v, %v", active, err)
	}

	encounter.Combatants = []models.Combatant{
		{CombatantID: "hero", Name: "Thistle", Type: "player", Initiative: 18,
			HitPoints: models.HitPoints{Current: 20, Maximum: 20}, Status: models.CombatantActive},
	}
	encounter.CombatState.Phase = models.PhaseActive
	encounter.CombatState.Round = 2
	encounter.DamageLog = []models.DamageLogEntry{
		{Attacker: "Thistle", Target: "Goblin", Damage: 9, DamageType: models.DamagePiercing, Round: 1},
	}
	encounter.NarrativeLog = []models.NarrativeLogEntry{{Text: "The rapier finds a gap.", Round: 1}}

	if err := encounters.SaveEncounter(ctx(), campaign.CampaignID, encounter); err != nil {
		t.Fatalf("SaveEncounter: %v", err)
	}

	reloaded, _ := encounters.GetEncounterInCampaign(ctx(), campaign.CampaignID, encounter.ID)
	if len(reloaded.Combatants) != 1 || reloaded.CombatState.Round != 2 {
		t.Errorf("state did not round-trip: %+v", reloaded.CombatState)
	}
	if len(reloaded.DamageLog) != 1 || len(reloaded.NarrativeLog) != 1 {
		t.Errorf("logs did not round-trip: %d damage, %d narration",
			len(reloaded.DamageLog), len(reloaded.NarrativeLog))
	}
	if reloaded.EncounterID != encounter.EncounterID {
		t.Error("the business id was blanked by the save")
	}

	stats := Stats(reloaded)
	if stats.TotalDamage != 9 || stats.DamageBySource["Thistle"] != 9 {
		t.Errorf("stats = %+v", stats)
	}

	// A finished encounter is no longer the active one.
	reloaded.CombatState.Phase = models.PhaseEnded
	if err := encounters.SaveEncounter(ctx(), campaign.CampaignID, reloaded); err != nil {
		t.Fatalf("SaveEncounter: %v", err)
	}
	if done, _ := encounters.GetActiveEncounter(ctx(), campaign.CampaignID); done != nil {
		t.Error("a finished encounter is still reported as active")
	}
}

// --- indexes ----------------------------------------------------------------

// The unique indexes are what several repository behaviours rely on, so their
// absence would show up as a much stranger bug later.
func TestUniqueIndexesAreEnforced(t *testing.T) {
	db := testDB(t)
	campaigns := NewCampaignRepository(db)

	first := newCampaign(t, campaigns)

	duplicate := &models.Campaign{
		Title: "Clash", CreatedBy: "dm1", CampaignID: first.CampaignID,
	}
	if err := campaigns.CreateCampaign(ctx(), duplicate); err == nil {
		t.Error("a duplicate campaign_id was accepted")
	}
}

func TestStartupIsIdempotent(t *testing.T) {
	client := testDB(t)

	// The server runs both on every boot; a second run must not fail.
	if err := client.InitializeCollections(ctx()); err != nil {
		t.Errorf("re-initialising collections: %v", err)
	}
	if err := client.CreateIndexes(ctx()); err != nil {
		t.Errorf("re-creating indexes: %v", err)
	}
}

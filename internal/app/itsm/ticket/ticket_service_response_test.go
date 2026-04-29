package ticket

import (
	"encoding/json"
	"testing"

	. "metis/internal/app/itsm/domain"
	"metis/internal/database"
	"metis/internal/model"
)

func TestBuildResponses_IncludesIntakeFormSchema(t *testing.T) {
	db := newTestDB(t)
	svc := &TicketService{ticketRepo: &TicketRepo{db: &database.DB{DB: db}}}

	user := model.User{Username: "schema-viewer", IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	catalog := ServiceCatalog{Name: "IT", Code: "it"}
	if err := db.Create(&catalog).Error; err != nil {
		t.Fatalf("create catalog: %v", err)
	}
	intakeSchema := JSONField(`{"version":1,"fields":[{"key":"access_window","type":"date_range","label":"访问时段","props":{"withTime":true,"mode":"datetime"}}]}`)
	service := ServiceDefinition{
		Name:             "Server Access",
		Code:             "server-access",
		CatalogID:        catalog.ID,
		EngineType:       "smart",
		IsActive:         true,
		IntakeFormSchema: intakeSchema,
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	priority := Priority{Name: "P1", Code: "p1", Value: 1, Color: "#f00", IsActive: true}
	if err := db.Create(&priority).Error; err != nil {
		t.Fatalf("create priority: %v", err)
	}
	ticket := Ticket{
		Code:        "TICK-SCHEMA",
		Title:       "Temporary access",
		ServiceID:   service.ID,
		EngineType:  "smart",
		Status:      TicketStatusSubmitted,
		PriorityID:  priority.ID,
		RequesterID: user.ID,
		Source:      TicketSourceCatalog,
		SLAStatus:   SLAStatusOnTrack,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	responses, err := svc.BuildResponses([]Ticket{ticket}, user.ID)
	if err != nil {
		t.Fatalf("BuildResponses: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if string(responses[0].IntakeFormSchema) != string(intakeSchema) {
		t.Fatalf("expected intake schema %s, got %s", intakeSchema, responses[0].IntakeFormSchema)
	}

	var payload map[string]any
	if err := json.Unmarshal(responses[0].IntakeFormSchema, &payload); err != nil {
		t.Fatalf("unmarshal intake schema: %v", err)
	}
	if payload["version"] != float64(1) {
		t.Fatalf("unexpected intake schema payload: %+v", payload)
	}
}

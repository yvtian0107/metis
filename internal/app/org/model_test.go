package org

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestJSONMap_Value(t *testing.T) {
	empty := JSONMap{}
	v, err := empty.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "{}" {
		t.Fatalf("expected {}, got %v", v)
	}

	nonEmpty := JSONMap(`{"key":"value"}`)
	v, err = nonEmpty.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != `{"key":"value"}` {
		t.Fatalf("unexpected value: %v", v)
	}
}

func TestJSONMap_Scan(t *testing.T) {
	var j JSONMap

	if err := j.Scan(`{"a":1}`); err != nil {
		t.Fatalf("scan string failed: %v", err)
	}
	if string(j) != `{"a":1}` {
		t.Fatalf("expected {\"a\":1}, got %s", string(j))
	}

	if err := j.Scan([]byte(`{"b":2}`)); err != nil {
		t.Fatalf("scan bytes failed: %v", err)
	}
	if string(j) != `{"b":2}` {
		t.Fatalf("expected {\"b\":2}, got %s", string(j))
	}

	if err := j.Scan(nil); err != nil {
		t.Fatalf("scan nil failed: %v", err)
	}
	if string(j) != "{}" {
		t.Fatalf("expected {}, got %s", string(j))
	}

	if err := j.Scan(123); err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestJSONMap_MarshalJSON(t *testing.T) {
	empty := JSONMap{}
	b, err := empty.MarshalJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(b, []byte("{}")) {
		t.Fatalf("expected {}, got %s", string(b))
	}

	j := JSONMap(`{"x":1}`)
	b, err = j.MarshalJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(b, []byte(`{"x":1}`)) {
		t.Fatalf("unexpected value: %s", string(b))
	}
}

func TestJSONMap_UnmarshalJSON(t *testing.T) {
	var j JSONMap
	if err := j.UnmarshalJSON([]byte(`{"y":2}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(j) != `{"y":2}` {
		t.Fatalf("unexpected value: %s", string(j))
	}
}

func TestJSONMap_RoundTrip(t *testing.T) {
	type Container struct {
		Data JSONMap `json:"data"`
	}

	original := Container{Data: JSONMap(`{"nested":{"arr":[1,2,3]}}`)}
	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed Container
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if string(original.Data) != string(parsed.Data) {
		t.Fatalf("round-trip mismatch: %s vs %s", string(original.Data), string(parsed.Data))
	}
}

func TestDepartment_ToResponse(t *testing.T) {
	now := time.Now()
	pid := uint(10)
	mid := uint(20)
	d := &Department{
		Name:        "Engineering",
		Code:        "eng",
		ParentID:    &pid,
		ManagerID:   &mid,
		Sort:        5,
		Description: "R&D",
		IsActive:    true,
	}
	d.ID = 1
	d.CreatedAt = now
	d.UpdatedAt = now

	r := d.ToResponse()
	if r.ID != 1 || r.Name != "Engineering" || r.Code != "eng" || r.Sort != 5 || r.Description != "R&D" || !r.IsActive {
		t.Fatalf("response mismatch: %+v", r)
	}
	if r.ParentID == nil || *r.ParentID != 10 {
		t.Fatal("expected parentId 10")
	}
	if r.ManagerID == nil || *r.ManagerID != 20 {
		t.Fatal("expected managerId 20")
	}
	if !r.CreatedAt.Equal(now) {
		t.Fatal("createdAt mismatch")
	}
}

func TestPosition_ToResponse(t *testing.T) {
	now := time.Now()
	p := &Position{
		Name:        "Senior Engineer",
		Code:        "se",
		Description: "L5",
		IsActive:    true,
	}
	p.ID = 2
	p.CreatedAt = now
	p.UpdatedAt = now

	r := p.ToResponse()
	if r.ID != 2 || r.Name != "Senior Engineer" || r.Code != "se" || r.Description != "L5" || !r.IsActive {
		t.Fatalf("response mismatch: %+v", r)
	}
	if !r.CreatedAt.Equal(now) {
		t.Fatal("createdAt mismatch")
	}
}

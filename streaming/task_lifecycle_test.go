package streaming

import (
	"encoding/json"
	"testing"
)

func TestTaskLifecycleEventID(t *testing.T) {
	plan := TaskPlan{PlanID: "plan-1", Execution: "sequential"}
	if got := TaskPlanLifecycle(TaskPlanUpdated, plan).EventID(); got != "task-plan-updated:plan-1" {
		t.Fatalf("plan event id = %q", got)
	}

	task := PlannedTask{ID: "task-1", Title: "Extract resume"}
	if got := TaskLifecycle(TaskBlocked, task).EventID(); got != "task-blocked:task-1" {
		t.Fatalf("task event id = %q", got)
	}
}

func TestTaskLifecycleChunkShape(t *testing.T) {
	task := PlannedTask{ID: "task-1", Title: "Extract resume"}
	result := TaskResult{
		TaskID:  "task-1",
		Status:  TaskResultAlternatePath,
		Summary: "Use converted PDF",
		Blocker: "Waiting for approval",
	}
	chunk := TaskResultLifecycle(TaskCompleted, task, result).Chunk()

	if chunk["type"] != "data-task-event" {
		t.Fatalf("chunk type = %q", chunk["type"])
	}
	if chunk["id"] != "task-completed:task-1" {
		t.Fatalf("chunk id = %q", chunk["id"])
	}

	bytes, err := json.Marshal(chunk["data"])
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(bytes, &data); err != nil {
		t.Fatal(err)
	}
	if data["event"] != "task-completed" {
		t.Fatalf("data event = %q", data["event"])
	}
	resultData := data["result"].(map[string]any)
	if resultData["status"] != "alternate_path" || resultData["summary"] != "Use converted PDF" {
		t.Fatalf("result = %#v", resultData)
	}
}

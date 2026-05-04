package streaming

type TaskLifecycleEvent string

const (
	TaskPlanCreated TaskLifecycleEvent = "task-plan-created"
	TaskPlanUpdated TaskLifecycleEvent = "task-plan-updated"
	TaskStarted     TaskLifecycleEvent = "task-started"
	TaskCompleted   TaskLifecycleEvent = "task-completed"
	TaskFailed      TaskLifecycleEvent = "task-failed"
	TaskSkipped     TaskLifecycleEvent = "task-skipped"
	TaskBlocked     TaskLifecycleEvent = "task-blocked"
)

type TaskStatus string

const (
	TaskStatusPlanned   TaskStatus = "planned"
	TaskStatusActive    TaskStatus = "active"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusSkipped   TaskStatus = "skipped"
	TaskStatusBlocked   TaskStatus = "blocked"
)

type TaskResultStatus string

const (
	TaskResultComplete      TaskResultStatus = "complete"
	TaskResultBlocked       TaskResultStatus = "blocked"
	TaskResultNeedsUser     TaskResultStatus = "needs_user"
	TaskResultAlternatePath TaskResultStatus = "alternate_path"
)

type PlannedTask struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Objective  string   `json:"objective,omitempty"`
	SkillNames []string `json:"skillNames,omitempty"`
	DependsOn  []string `json:"dependsOn,omitempty"`
	Execution  string   `json:"execution,omitempty"`
}

type TaskPlan struct {
	PlanID            string        `json:"planId,omitempty"`
	Summary           string        `json:"summary,omitempty"`
	Execution         string        `json:"execution"`
	Reason            string        `json:"reason,omitempty"`
	RequiresSynthesis bool          `json:"requiresSynthesis,omitempty"`
	Tasks             []PlannedTask `json:"tasks,omitempty"`
}

type TaskResult struct {
	TaskID  string           `json:"taskId"`
	Title   string           `json:"title,omitempty"`
	Text    string           `json:"text,omitempty"`
	Status  TaskResultStatus `json:"status,omitempty"`
	Summary string           `json:"summary,omitempty"`
	Blocker string           `json:"blocker,omitempty"`
}

type TaskLifecycleData struct {
	Event          TaskLifecycleEvent `json:"event"`
	ID             string             `json:"id,omitempty"`
	Plan           *TaskPlan          `json:"plan,omitempty"`
	Task           *PlannedTask       `json:"task,omitempty"`
	Result         *TaskResult        `json:"result,omitempty"`
	Error          string             `json:"error,omitempty"`
	Reason         string             `json:"reason,omitempty"`
	BlockedTaskIDs []string           `json:"blockedTaskIds,omitempty"`
	SkippedTaskIDs []string           `json:"skippedTaskIds,omitempty"`
}

func TaskPlanLifecycle(event TaskLifecycleEvent, plan TaskPlan) TaskLifecycleData {
	return TaskLifecycleData{Event: event, Plan: &plan}
}

func TaskLifecycle(event TaskLifecycleEvent, task PlannedTask) TaskLifecycleData {
	return TaskLifecycleData{Event: event, Task: &task}
}

func TaskResultLifecycle(event TaskLifecycleEvent, task PlannedTask, result TaskResult) TaskLifecycleData {
	return TaskLifecycleData{Event: event, Task: &task, Result: &result}
}

func TaskErrorLifecycle(event TaskLifecycleEvent, task PlannedTask, err error) TaskLifecycleData {
	data := TaskLifecycleData{Event: event, Task: &task}
	if err != nil {
		data.Error = err.Error()
	}
	return data
}

func (data TaskLifecycleData) EventID() string {
	switch data.Event {
	case TaskPlanCreated, TaskPlanUpdated:
		if data.Plan != nil && data.Plan.PlanID != "" {
			return string(data.Event) + ":" + data.Plan.PlanID
		}
	case TaskStarted, TaskCompleted, TaskFailed, TaskSkipped, TaskBlocked:
		if data.Task != nil && data.Task.ID != "" {
			return string(data.Event) + ":" + data.Task.ID
		}
	}
	if data.ID != "" {
		return data.ID
	}
	return string(data.Event)
}

func (data TaskLifecycleData) Chunk() map[string]any {
	return map[string]any{
		"type": "data-task-event",
		"id":   data.EventID(),
		"data": data,
	}
}

package temporalai

import (
	"github.com/holbrookab/go-temporal-ai-sdk/activities"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
)

func RegisterActivities(w worker.Worker, acts *activities.Activities) {
	w.RegisterActivityWithOptions(acts.InvokeModel, activity.RegisterOptions{Name: activities.InvokeModelActivity})
	w.RegisterActivityWithOptions(acts.InvokeModelStream, activity.RegisterOptions{Name: activities.InvokeModelStreamActivity})
	w.RegisterActivityWithOptions(acts.InvokeEmbeddingModel, activity.RegisterOptions{Name: activities.InvokeEmbeddingModelActivity})
	w.RegisterActivityWithOptions(acts.PublishToolLifecycleEvent, activity.RegisterOptions{Name: activities.PublishToolLifecycleEventActivity})
}

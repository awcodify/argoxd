package argocd

import (
	"context"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// demoEvent is a sample event that happened the given time ago.
func demoEvent(kind, name, eventType, reason, message string, count int32, ago time.Duration) corev1.Event {
	return corev1.Event{
		InvolvedObject: corev1.ObjectReference{Kind: kind, Name: name},
		Type:           eventType, Reason: reason, Message: message, Count: count,
		LastTimestamp: metav1.NewTime(time.Now().Add(-ago)),
	}
}

// ApplicationEvents returns sample events of an Application.
func (s *DemoSource) ApplicationEvents(_ context.Context, name string) (string, error) {
	application, err := s.find(name)
	if err != nil {
		return "", err
	}
	events := []corev1.Event{
		demoEvent("Application", name, corev1.EventTypeNormal, "ResourceUpdated", "Updated health status: Progressing -> "+application.Health, 1, 4*time.Hour),
		demoEvent("Application", name, corev1.EventTypeNormal, "OperationCompleted", "Sync operation to "+application.Revision+" succeeded", 1, 3*time.Hour),
		demoEvent("Application", name, corev1.EventTypeNormal, "ResourceUpdated", "Updated sync status: "+application.Sync, 1, 2*time.Minute),
	}
	return formatEvents(events, time.Now()), nil
}

// ResourceEvents returns sample events for a resource: a Degraded Pod keeps
// restarting, anything else rolled out cleanly.
func (s *DemoSource) ResourceEvents(_ context.Context, _ string, resource explorer.ResourceNode) (string, error) {
	var events []corev1.Event
	switch {
	case resource.Kind == "Pod" && resource.Health == "Degraded":
		events = []corev1.Event{
			demoEvent("Pod", resource.Name, corev1.EventTypeNormal, "Scheduled", "Successfully assigned "+resource.Namespace+"/"+resource.Name+" to node-2", 1, 5*time.Hour),
			demoEvent("Pod", resource.Name, corev1.EventTypeNormal, "Pulled", "Container image \"registry.example.com/app:1.4.2\" already present on machine", 6, 40*time.Minute),
			demoEvent("Pod", resource.Name, corev1.EventTypeWarning, "Unhealthy", "Readiness probe failed: HTTP probe failed with statuscode: 503", 12, 3*time.Minute),
			demoEvent("Pod", resource.Name, corev1.EventTypeWarning, "BackOff", "Back-off restarting failed container app in pod "+resource.Name, 31, time.Minute),
		}
	case resource.Kind == "Pod":
		events = []corev1.Event{
			demoEvent("Pod", resource.Name, corev1.EventTypeNormal, "Scheduled", "Successfully assigned "+resource.Namespace+"/"+resource.Name+" to node-1", 1, 26*time.Hour),
			demoEvent("Pod", resource.Name, corev1.EventTypeNormal, "Started", "Started container app", 1, 26*time.Hour),
		}
	default:
		events = []corev1.Event{
			demoEvent(resource.Kind, resource.Name, corev1.EventTypeNormal, "ScalingReplicaSet", "Scaled up replica set "+resource.Name+" to 3", 1, 26*time.Hour),
		}
	}
	return formatEvents(events, time.Now()), nil
}

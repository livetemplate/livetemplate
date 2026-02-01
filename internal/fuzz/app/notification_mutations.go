package app

import (
	"fmt"
	"math/rand"

	"github.com/livetemplate/livetemplate/internal/fuzz/mutations"
)

var notificationCounter int

// ApplyNotificationMutation applies a mutation to the NotificationState.
// Returns an error if the mutation cannot be applied.
func ApplyNotificationMutation(state *NotificationState, mutation mutations.Mutation) error {
	switch mutation.Type {
	case mutations.MutAddNotification:
		notif, ok := mutation.Value.(Notification)
		if !ok {
			if m, ok := mutation.Value.(map[string]any); ok {
				notif = notificationFromMap(m)
			} else {
				return fmt.Errorf("MutAddNotification requires Notification value, got %T", mutation.Value)
			}
		}
		state.Notifications = append(state.Notifications, notif)

	case mutations.MutDismissNotification:
		if mutation.Target == "" {
			return fmt.Errorf("MutDismissNotification requires notification ID in Target")
		}
		for i, notif := range state.Notifications {
			if notif.ID == mutation.Target {
				state.Notifications = append(state.Notifications[:i], state.Notifications[i+1:]...)
				return nil
			}
		}
		// Notification not found - silently ignore (may have already been dismissed)

	case mutations.MutDismissAll:
		state.Notifications = nil

	case mutations.MutExpireNotification:
		// Find a notification with AutoDismiss=true and Timer > 0, decrement timer
		// If timer reaches 0, remove the notification
		if mutation.Target == "" {
			return fmt.Errorf("MutExpireNotification requires notification ID in Target")
		}
		for i, notif := range state.Notifications {
			if notif.ID == mutation.Target && notif.AutoDismiss {
				if notif.Timer > 0 {
					state.Notifications[i].Timer--
					if state.Notifications[i].Timer == 0 {
						state.Notifications = append(state.Notifications[:i], state.Notifications[i+1:]...)
					}
				}
				return nil
			}
		}

	case mutations.MutToggleBool:
		// No boolean fields to toggle in NotificationState
		// MaxVisible is an int, not bool

	case mutations.MutAppendSlice:
		notif, ok := mutation.Value.(Notification)
		if !ok {
			if m, ok := mutation.Value.(map[string]any); ok {
				notif = notificationFromMap(m)
			} else {
				return fmt.Errorf("MutAppendSlice requires Notification value, got %T", mutation.Value)
			}
		}
		state.Notifications = append(state.Notifications, notif)

	case mutations.MutRemoveSlice:
		if mutation.Index < 0 || mutation.Index >= len(state.Notifications) {
			return fmt.Errorf("index %d out of range [0, %d)", mutation.Index, len(state.Notifications))
		}
		state.Notifications = append(state.Notifications[:mutation.Index], state.Notifications[mutation.Index+1:]...)

	case mutations.MutClearSlice:
		state.Notifications = nil

	default:
		return fmt.Errorf("unsupported mutation type for NotificationState: %s", mutation.Type)
	}

	return nil
}

func notificationFromMap(m map[string]any) Notification {
	notif := Notification{}
	if v, ok := m["ID"].(string); ok {
		notif.ID = v
	}
	if v, ok := m["Message"].(string); ok {
		notif.Message = v
	}
	if v, ok := m["Type"].(string); ok {
		notif.Type = NotificationType(v)
	}
	if v, ok := m["Type"].(NotificationType); ok {
		notif.Type = v
	}
	if v, ok := m["AutoDismiss"].(bool); ok {
		notif.AutoDismiss = v
	}
	if v, ok := m["Timer"].(int); ok {
		notif.Timer = v
	}
	return notif
}

// GenNotificationMutation generates a random mutation for NotificationState based on weights.
func GenNotificationMutation(rng *rand.Rand, state *NotificationState, weights mutations.MutationWeights) mutations.Mutation {
	type weightedMutation struct {
		mutType mutations.MutationType
		weight  float64
	}

	options := []weightedMutation{
		{mutations.MutAddNotification, weights.AddNotification},
		{mutations.MutDismissNotification, weights.DismissNotification},
		{mutations.MutDismissAll, weights.DismissAll},
		{mutations.MutExpireNotification, weights.ExpireNotification},
		{mutations.MutAppendSlice, weights.AppendSlice},
		{mutations.MutRemoveSlice, weights.RemoveSlice},
	}

	choice := rng.Float64()
	cumulative := 0.0
	var selectedType mutations.MutationType

	for _, opt := range options {
		cumulative += opt.weight
		if choice < cumulative {
			selectedType = opt.mutType
			break
		}
	}

	if selectedType == "" {
		selectedType = mutations.MutAddNotification // Fallback
	}

	return genNotificationMutationOfType(rng, state, selectedType)
}

func genNotificationMutationOfType(rng *rand.Rand, state *NotificationState, mutType mutations.MutationType) mutations.Mutation {
	switch mutType {
	case mutations.MutAddNotification:
		return mutations.Mutation{
			Type:  mutations.MutAddNotification,
			Value: genNotification(rng),
		}

	case mutations.MutDismissNotification:
		if len(state.Notifications) == 0 {
			return genNotificationMutationOfType(rng, state, mutations.MutAddNotification)
		}
		notif := state.Notifications[rng.Intn(len(state.Notifications))]
		return mutations.Mutation{
			Type:   mutations.MutDismissNotification,
			Target: notif.ID,
		}

	case mutations.MutDismissAll:
		return mutations.Mutation{Type: mutations.MutDismissAll}

	case mutations.MutExpireNotification:
		// Find notifications with AutoDismiss=true and Timer > 0
		var expirable []Notification
		for _, notif := range state.Notifications {
			if notif.AutoDismiss && notif.Timer > 0 {
				expirable = append(expirable, notif)
			}
		}
		if len(expirable) == 0 {
			// Add an auto-dismiss notification first
			return mutations.Mutation{
				Type:  mutations.MutAddNotification,
				Value: genAutoDismissNotification(rng),
			}
		}
		return mutations.Mutation{
			Type:   mutations.MutExpireNotification,
			Target: expirable[rng.Intn(len(expirable))].ID,
		}

	case mutations.MutAppendSlice:
		return mutations.Mutation{
			Type:  mutations.MutAppendSlice,
			Value: genNotification(rng),
		}

	case mutations.MutRemoveSlice:
		if len(state.Notifications) == 0 {
			return genNotificationMutationOfType(rng, state, mutations.MutAddNotification)
		}
		return mutations.Mutation{
			Type:  mutations.MutRemoveSlice,
			Index: rng.Intn(len(state.Notifications)),
		}

	default:
		return mutations.Mutation{
			Type:  mutations.MutAddNotification,
			Value: genNotification(rng),
		}
	}
}

func genNotification(rng *rand.Rand) Notification {
	notificationCounter++
	autoDismiss := rng.Float32() > 0.5
	timer := 0
	if autoDismiss {
		timer = 3 + rng.Intn(5) // 3-7 seconds
	}
	return Notification{
		ID:          fmt.Sprintf("notif-%d", notificationCounter),
		Message:     genNotificationMessage(rng),
		Type:        NotificationTypeValues[rng.Intn(len(NotificationTypeValues))],
		AutoDismiss: autoDismiss,
		Timer:       timer,
	}
}

func genAutoDismissNotification(rng *rand.Rand) Notification {
	notificationCounter++
	return Notification{
		ID:          fmt.Sprintf("notif-%d", notificationCounter),
		Message:     genNotificationMessage(rng),
		Type:        NotificationTypeValues[rng.Intn(len(NotificationTypeValues))],
		AutoDismiss: true,
		Timer:       3 + rng.Intn(5), // 3-7 seconds
	}
}

func genNotificationMessage(rng *rand.Rand) string {
	messages := []string{
		"Item saved successfully",
		"Changes applied",
		"Warning: Low disk space",
		"Error: Connection failed",
		"Welcome back!",
		"Your profile was updated",
		"New message received",
		"Task completed",
		"Session expires in 5 minutes",
		"File uploaded successfully",
	}
	return messages[rng.Intn(len(messages))]
}

// GenNotificationState generates a random NotificationState for testing.
func GenNotificationState(rng *rand.Rand) *NotificationState {
	state := DefaultNotificationState()

	// Generate 0-8 notifications
	numNotifications := rng.Intn(9)
	state.Notifications = make([]Notification, numNotifications)
	for i := range state.Notifications {
		state.Notifications[i] = genNotification(rng)
	}

	// Random max visible (3-10)
	state.MaxVisible = 3 + rng.Intn(8)

	return state
}

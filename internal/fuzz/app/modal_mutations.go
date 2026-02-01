package app

import (
	"fmt"
	"math/rand"

	"github.com/livetemplate/livetemplate/internal/fuzz/mutations"
)

var modalCounter int

// ApplyModalMutation applies a mutation to the ModalState.
// Returns an error if the mutation cannot be applied.
func ApplyModalMutation(state *ModalState, mutation mutations.Mutation) error {
	switch mutation.Type {
	case mutations.MutOpenModal:
		modal, ok := mutation.Value.(Modal)
		if !ok {
			if m, ok := mutation.Value.(map[string]any); ok {
				modal = modalFromMap(m)
			} else {
				return fmt.Errorf("MutOpenModal requires Modal value, got %T", mutation.Value)
			}
		}
		modal.IsOpen = true
		state.Modals = append(state.Modals, modal)

	case mutations.MutCloseModal:
		if mutation.Index < 0 || mutation.Index >= len(state.Modals) {
			return fmt.Errorf("modal index %d out of range [0, %d)", mutation.Index, len(state.Modals))
		}
		// Close the modal (mark as closed rather than remove)
		state.Modals[mutation.Index].IsOpen = false

	case mutations.MutCloseAll:
		for i := range state.Modals {
			state.Modals[i].IsOpen = false
		}

	case mutations.MutUpdateModal:
		if mutation.Index < 0 || mutation.Index >= len(state.Modals) {
			return fmt.Errorf("modal index %d out of range [0, %d)", mutation.Index, len(state.Modals))
		}
		updates, ok := mutation.Value.(map[string]any)
		if !ok {
			return fmt.Errorf("MutUpdateModal requires map[string]any updates, got %T", mutation.Value)
		}
		modal := &state.Modals[mutation.Index]
		if v, ok := updates["Title"].(string); ok {
			modal.Title = v
		}
		if v, ok := updates["Content"].(string); ok {
			modal.Content = v
		}
		if v, ok := updates["IsOpen"].(bool); ok {
			modal.IsOpen = v
		}
		if v, ok := updates["Priority"].(int); ok {
			modal.Priority = v
		}

	case mutations.MutSwitchPanel:
		panel, ok := mutation.Value.(string)
		if !ok {
			return fmt.Errorf("MutSwitchPanel requires string value, got %T", mutation.Value)
		}
		state.ActivePanel = panel

	case mutations.MutTogglePanel:
		if state.ActivePanel == "none" {
			// Pick a random panel to open
			panels := []string{"left", "right", "bottom"}
			state.ActivePanel = panels[mutation.Index%len(panels)]
		} else {
			state.ActivePanel = "none"
		}

	case mutations.MutReorderModal:
		if len(state.Modals) < 2 {
			return fmt.Errorf("need at least 2 modals to reorder")
		}
		if mutation.Index < 0 || mutation.Index >= len(state.Modals) {
			return fmt.Errorf("modal index %d out of range", mutation.Index)
		}
		if mutation.Index2 < 0 || mutation.Index2 >= len(state.Modals) {
			return fmt.Errorf("modal index2 %d out of range", mutation.Index2)
		}
		// Swap modals
		state.Modals[mutation.Index], state.Modals[mutation.Index2] = state.Modals[mutation.Index2], state.Modals[mutation.Index]

	case mutations.MutRemoveSlice:
		// Remove a modal from the stack entirely
		if mutation.Index < 0 || mutation.Index >= len(state.Modals) {
			return fmt.Errorf("modal index %d out of range [0, %d)", mutation.Index, len(state.Modals))
		}
		state.Modals = append(state.Modals[:mutation.Index], state.Modals[mutation.Index+1:]...)

	case mutations.MutClearSlice:
		state.Modals = nil

	default:
		return fmt.Errorf("unsupported mutation type for ModalState: %s", mutation.Type)
	}

	return nil
}

func modalFromMap(m map[string]any) Modal {
	modal := Modal{}
	if v, ok := m["ID"].(string); ok {
		modal.ID = v
	}
	if v, ok := m["Title"].(string); ok {
		modal.Title = v
	}
	if v, ok := m["Content"].(string); ok {
		modal.Content = v
	}
	if v, ok := m["IsOpen"].(bool); ok {
		modal.IsOpen = v
	}
	if v, ok := m["Priority"].(int); ok {
		modal.Priority = v
	}
	return modal
}

// GenModalMutation generates a random mutation for ModalState based on weights.
func GenModalMutation(rng *rand.Rand, state *ModalState, weights mutations.MutationWeights) mutations.Mutation {
	type weightedMutation struct {
		mutType mutations.MutationType
		weight  float64
	}

	options := []weightedMutation{
		{mutations.MutOpenModal, weights.OpenModal},
		{mutations.MutCloseModal, weights.CloseModal},
		{mutations.MutCloseAll, weights.CloseAll},
		{mutations.MutUpdateModal, weights.UpdateModal},
		{mutations.MutSwitchPanel, weights.SwitchPanel},
		{mutations.MutTogglePanel, weights.TogglePanel},
		{mutations.MutReorderModal, weights.ReorderModal},
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
		selectedType = mutations.MutOpenModal // Fallback
	}

	return genModalMutationOfType(rng, state, selectedType)
}

func genModalMutationOfType(rng *rand.Rand, state *ModalState, mutType mutations.MutationType) mutations.Mutation {
	switch mutType {
	case mutations.MutOpenModal:
		return mutations.Mutation{
			Type:  mutations.MutOpenModal,
			Value: genModal(rng),
		}

	case mutations.MutCloseModal:
		openModals := findOpenModals(state)
		if len(openModals) == 0 {
			return genModalMutationOfType(rng, state, mutations.MutOpenModal)
		}
		return mutations.Mutation{
			Type:  mutations.MutCloseModal,
			Index: openModals[rng.Intn(len(openModals))],
		}

	case mutations.MutCloseAll:
		if !hasOpenModal(state) {
			return genModalMutationOfType(rng, state, mutations.MutOpenModal)
		}
		return mutations.Mutation{Type: mutations.MutCloseAll}

	case mutations.MutUpdateModal:
		if len(state.Modals) == 0 {
			return genModalMutationOfType(rng, state, mutations.MutOpenModal)
		}
		updates := make(map[string]any)
		if rng.Float32() > 0.5 {
			updates["Title"] = genModalTitle(rng)
		}
		if rng.Float32() > 0.5 {
			updates["Content"] = genModalContent(rng)
		}
		if len(updates) == 0 {
			updates["Title"] = genModalTitle(rng)
		}
		return mutations.Mutation{
			Type:  mutations.MutUpdateModal,
			Index: rng.Intn(len(state.Modals)),
			Value: updates,
		}

	case mutations.MutSwitchPanel:
		panels := []string{"none", "left", "right", "bottom"}
		// Pick a different panel than current
		newPanel := panels[rng.Intn(len(panels))]
		for newPanel == state.ActivePanel && len(panels) > 1 {
			newPanel = panels[rng.Intn(len(panels))]
		}
		return mutations.Mutation{
			Type:  mutations.MutSwitchPanel,
			Value: newPanel,
		}

	case mutations.MutTogglePanel:
		return mutations.Mutation{
			Type:  mutations.MutTogglePanel,
			Index: rng.Intn(3), // Used to pick panel if opening
		}

	case mutations.MutReorderModal:
		if len(state.Modals) < 2 {
			return genModalMutationOfType(rng, state, mutations.MutOpenModal)
		}
		idx1 := rng.Intn(len(state.Modals))
		idx2 := rng.Intn(len(state.Modals))
		for idx2 == idx1 {
			idx2 = rng.Intn(len(state.Modals))
		}
		return mutations.Mutation{
			Type:   mutations.MutReorderModal,
			Index:  idx1,
			Index2: idx2,
		}

	default:
		return mutations.Mutation{
			Type:  mutations.MutOpenModal,
			Value: genModal(rng),
		}
	}
}

func findOpenModals(state *ModalState) []int {
	var indices []int
	for i, modal := range state.Modals {
		if modal.IsOpen {
			indices = append(indices, i)
		}
	}
	return indices
}

func hasOpenModal(state *ModalState) bool {
	for _, modal := range state.Modals {
		if modal.IsOpen {
			return true
		}
	}
	return false
}

func genModal(rng *rand.Rand) Modal {
	modalCounter++
	return Modal{
		ID:       fmt.Sprintf("modal-%d", modalCounter),
		Title:    genModalTitle(rng),
		Content:  genModalContent(rng),
		IsOpen:   true,
		Priority: rng.Intn(10),
	}
}

func genModalTitle(rng *rand.Rand) string {
	titles := []string{
		"Confirm Action",
		"Settings",
		"Help",
		"Warning",
		"Error",
		"Success",
		"Edit Item",
		"New Item",
		"Delete Confirmation",
		"Details",
	}
	return titles[rng.Intn(len(titles))]
}

func genModalContent(rng *rand.Rand) string {
	contents := []string{
		"Are you sure you want to continue?",
		"Please review the following information.",
		"Click OK to proceed or Cancel to go back.",
		"This action cannot be undone.",
		"Operation completed successfully.",
		"An error occurred. Please try again.",
		"Loading data...",
		"Processing your request...",
	}
	return contents[rng.Intn(len(contents))]
}

// GenModalState generates a random ModalState for testing.
func GenModalState(rng *rand.Rand) *ModalState {
	state := DefaultModalState()

	// Generate 0-3 modals
	numModals := rng.Intn(4)
	if numModals > 0 {
		state.Modals = make([]Modal, numModals)
		for i := range state.Modals {
			state.Modals[i] = genModal(rng)
			// Only the last modal is typically open
			state.Modals[i].IsOpen = i == numModals-1 && rng.Float32() > 0.3
		}
	}

	// Random panel state
	state.ActivePanel = PanelValues[rng.Intn(len(PanelValues))]

	return state
}

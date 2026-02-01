package app

import (
	"fmt"
	"math/rand"

	"github.com/livetemplate/livetemplate/internal/fuzz/mutations"
)

// ApplyFormMutation applies a mutation to the FormState.
// Returns an error if the mutation cannot be applied.
func ApplyFormMutation(state *FormState, mutation mutations.Mutation) error {
	switch mutation.Type {
	case mutations.MutSetFieldValue:
		fieldName, ok := mutation.Target, true
		if mutation.Target == "" {
			return fmt.Errorf("MutSetFieldValue requires field name in Target")
		}
		value, ok := mutation.Value.(string)
		if !ok {
			return fmt.Errorf("MutSetFieldValue requires string value, got %T", mutation.Value)
		}
		state.Fields[fieldName] = value

	case mutations.MutSetFieldError:
		if mutation.Target == "" {
			return fmt.Errorf("MutSetFieldError requires field name in Target")
		}
		errorMsg, ok := mutation.Value.(string)
		if !ok {
			return fmt.Errorf("MutSetFieldError requires string value, got %T", mutation.Value)
		}
		state.Errors[mutation.Target] = errorMsg

	case mutations.MutClearFieldError:
		if mutation.Target == "" {
			return fmt.Errorf("MutClearFieldError requires field name in Target")
		}
		delete(state.Errors, mutation.Target)

	case mutations.MutTouchField:
		if mutation.Target == "" {
			return fmt.Errorf("MutTouchField requires field name in Target")
		}
		state.Touched[mutation.Target] = true

	case mutations.MutResetForm:
		state.Fields = make(map[string]string)
		state.Errors = make(map[string]string)
		state.Touched = make(map[string]bool)
		state.Submitting = false
		state.Submitted = false

	case mutations.MutSubmitStart:
		state.Submitting = true
		state.Submitted = false

	case mutations.MutSubmitSuccess:
		state.Submitting = false
		state.Submitted = true
		// Clear errors on success
		state.Errors = make(map[string]string)

	case mutations.MutSubmitError:
		state.Submitting = false
		state.Submitted = false
		// Error message is set via SetFieldError mutations

	case mutations.MutToggleBool:
		// Toggle submitting or submitted based on Target
		switch mutation.Target {
		case "Submitting":
			state.Submitting = !state.Submitting
		case "Submitted":
			state.Submitted = !state.Submitted
		}

	default:
		return fmt.Errorf("unsupported mutation type for FormState: %s", mutation.Type)
	}

	return nil
}

// GenFormMutation generates a random mutation for FormState based on weights.
func GenFormMutation(rng *rand.Rand, state *FormState, weights mutations.MutationWeights) mutations.Mutation {
	type weightedMutation struct {
		mutType mutations.MutationType
		weight  float64
	}

	options := []weightedMutation{
		{mutations.MutSetFieldValue, weights.SetFieldValue},
		{mutations.MutSetFieldError, weights.SetFieldError},
		{mutations.MutClearFieldError, weights.ClearFieldError},
		{mutations.MutTouchField, weights.TouchField},
		{mutations.MutResetForm, weights.ResetForm},
		{mutations.MutSubmitStart, weights.SubmitStart},
		{mutations.MutSubmitSuccess, weights.SubmitSuccess},
		{mutations.MutSubmitError, weights.SubmitError},
		{mutations.MutToggleBool, weights.ToggleBool},
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
		selectedType = mutations.MutSetFieldValue // Fallback
	}

	return genFormMutationOfType(rng, state, selectedType)
}

func genFormMutationOfType(rng *rand.Rand, state *FormState, mutType mutations.MutationType) mutations.Mutation {
	switch mutType {
	case mutations.MutSetFieldValue:
		fieldName := FormFieldNames[rng.Intn(len(FormFieldNames))]
		return mutations.Mutation{
			Type:   mutations.MutSetFieldValue,
			Target: fieldName,
			Value:  genFieldValue(rng, fieldName),
		}

	case mutations.MutSetFieldError:
		fieldName := FormFieldNames[rng.Intn(len(FormFieldNames))]
		return mutations.Mutation{
			Type:   mutations.MutSetFieldError,
			Target: fieldName,
			Value:  genErrorMessage(rng, fieldName),
		}

	case mutations.MutClearFieldError:
		// Find a field with an error to clear
		var fieldsWithErrors []string
		for field, err := range state.Errors {
			if err != "" {
				fieldsWithErrors = append(fieldsWithErrors, field)
			}
		}
		if len(fieldsWithErrors) == 0 {
			// No errors to clear, set one instead
			return genFormMutationOfType(rng, state, mutations.MutSetFieldError)
		}
		return mutations.Mutation{
			Type:   mutations.MutClearFieldError,
			Target: fieldsWithErrors[rng.Intn(len(fieldsWithErrors))],
		}

	case mutations.MutTouchField:
		fieldName := FormFieldNames[rng.Intn(len(FormFieldNames))]
		return mutations.Mutation{
			Type:   mutations.MutTouchField,
			Target: fieldName,
		}

	case mutations.MutResetForm:
		return mutations.Mutation{Type: mutations.MutResetForm}

	case mutations.MutSubmitStart:
		return mutations.Mutation{Type: mutations.MutSubmitStart}

	case mutations.MutSubmitSuccess:
		if !state.Submitting {
			// Can only succeed if submitting
			return genFormMutationOfType(rng, state, mutations.MutSubmitStart)
		}
		return mutations.Mutation{Type: mutations.MutSubmitSuccess}

	case mutations.MutSubmitError:
		if !state.Submitting {
			// Can only error if submitting
			return genFormMutationOfType(rng, state, mutations.MutSubmitStart)
		}
		return mutations.Mutation{Type: mutations.MutSubmitError}

	case mutations.MutToggleBool:
		targets := []string{"Submitting", "Submitted"}
		return mutations.Mutation{
			Type:   mutations.MutToggleBool,
			Target: targets[rng.Intn(len(targets))],
		}

	default:
		return mutations.Mutation{
			Type:   mutations.MutSetFieldValue,
			Target: FormFieldNames[0],
			Value:  "default",
		}
	}
}

func genFieldValue(rng *rand.Rand, fieldName string) string {
	switch fieldName {
	case "name":
		names := []string{"John Doe", "Jane Smith", "Bob Wilson", "Alice Brown", ""}
		return names[rng.Intn(len(names))]
	case "email":
		emails := []string{"john@example.com", "jane@test.org", "invalid-email", "bob@", ""}
		return emails[rng.Intn(len(emails))]
	case "password":
		passwords := []string{"password123", "short", "ValidP@ssw0rd!", "12345", ""}
		return passwords[rng.Intn(len(passwords))]
	case "phone":
		phones := []string{"555-1234", "+1-555-555-5555", "not-a-phone", "123", ""}
		return phones[rng.Intn(len(phones))]
	case "address":
		addresses := []string{"123 Main St", "456 Oak Ave, Suite 100", "", "P.O. Box 789"}
		return addresses[rng.Intn(len(addresses))]
	default:
		return fmt.Sprintf("value-%d", rng.Intn(100))
	}
}

func genErrorMessage(rng *rand.Rand, fieldName string) string {
	switch fieldName {
	case "name":
		errors := []string{"Name is required", "Name must be at least 2 characters", "Name contains invalid characters"}
		return errors[rng.Intn(len(errors))]
	case "email":
		errors := []string{"Email is required", "Invalid email format", "Email already exists"}
		return errors[rng.Intn(len(errors))]
	case "password":
		errors := []string{"Password is required", "Password must be at least 8 characters", "Password must contain uppercase and number"}
		return errors[rng.Intn(len(errors))]
	case "phone":
		errors := []string{"Phone is required", "Invalid phone format", "Phone number too short"}
		return errors[rng.Intn(len(errors))]
	case "address":
		errors := []string{"Address is required", "Address is too long", "Invalid address format"}
		return errors[rng.Intn(len(errors))]
	default:
		return "Invalid value"
	}
}

// GenFormState generates a random FormState for testing.
func GenFormState(rng *rand.Rand) *FormState {
	state := DefaultFormState()

	// Randomly fill some fields
	for _, field := range FormFieldNames {
		if rng.Float32() > 0.3 { // 70% chance to have a value
			state.Fields[field] = genFieldValue(rng, field)
		}
		if rng.Float32() > 0.7 { // 30% chance to have an error
			state.Errors[field] = genErrorMessage(rng, field)
		}
		if rng.Float32() > 0.5 { // 50% chance to be touched
			state.Touched[field] = true
		}
	}

	// Random submit state
	if rng.Float32() > 0.8 {
		state.Submitting = true
	}
	if rng.Float32() > 0.9 {
		state.Submitted = true
	}

	return state
}

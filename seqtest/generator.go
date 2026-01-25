package seqtest

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"time"

	"github.com/livetemplate/livetemplate"
)

// DataGen generates valid data for an action given the current state
type DataGen func(rng *rand.Rand, currentState interface{}) map[string]interface{}

// ActionGenerator creates random valid actions for fuzz testing
type ActionGenerator struct {
	controller interface{}
	stateType  reflect.Type
	rng        *rand.Rand

	// Available actions discovered from controller
	actions []string

	// Optional constraints
	weights  map[string]float64  // Action frequency weights (default 1.0)
	dataGens map[string]DataGen  // Custom data generators per action
	skips    map[string]SkipFunc // Skip conditions per action
}

// SkipFunc determines if an action should be skipped given current state
type SkipFunc func(currentState interface{}) bool

// NewActionGenerator creates a new generator for the given controller and state type
func NewActionGenerator(controller interface{}, stateType reflect.Type) *ActionGenerator {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	gen := &ActionGenerator{
		controller: controller,
		stateType:  stateType,
		rng:        rng,
		weights:    make(map[string]float64),
		dataGens:   make(map[string]DataGen),
		skips:      make(map[string]SkipFunc),
	}

	// Discover available actions from controller
	gen.discoverActions()

	return gen
}

// WithSeed sets the random seed for reproducible generation
func (g *ActionGenerator) WithSeed(seed int64) *ActionGenerator {
	g.rng = rand.New(rand.NewSource(seed))
	return g
}

// SetWeight sets the selection weight for an action (default is 1.0)
func (g *ActionGenerator) SetWeight(action string, weight float64) {
	g.weights[action] = weight
}

// SetWeights sets multiple weights at once
func (g *ActionGenerator) SetWeights(weights map[string]float64) {
	for action, weight := range weights {
		g.weights[action] = weight
	}
}

// SetDataGen sets a custom data generator for an action
func (g *ActionGenerator) SetDataGen(action string, gen DataGen) {
	g.dataGens[action] = gen
}

// SetSkipFunc sets a skip condition for an action
func (g *ActionGenerator) SetSkipFunc(action string, skip SkipFunc) {
	g.skips[action] = skip
}

// Actions returns the list of discovered actions
func (g *ActionGenerator) Actions() []string {
	return g.actions
}

// Generate creates a random valid action based on current state
func (g *ActionGenerator) Generate(currentState interface{}) Action {
	if len(g.actions) == 0 {
		return Action{}
	}

	// Build list of available actions (respecting skip conditions)
	available := make([]string, 0, len(g.actions))
	for _, action := range g.actions {
		if skip, ok := g.skips[action]; ok {
			if skip(currentState) {
				continue
			}
		}
		available = append(available, action)
	}

	if len(available) == 0 {
		return Action{}
	}

	// Select action with weighted random
	selected := g.selectWeighted(available)

	// Generate data
	var data map[string]interface{}
	if gen, ok := g.dataGens[selected]; ok {
		data = gen(g.rng, currentState)
		if data == nil {
			// Generator returned nil, try another action
			return g.Generate(currentState)
		}
	}

	return Action{Name: selected, Data: data}
}

// GenerateN generates n random actions
func (g *ActionGenerator) GenerateN(n int, executor Executor) []Action {
	actions := make([]Action, 0, n)

	for i := 0; i < n; i++ {
		action := g.Generate(executor.CurrentState())
		if action.Name == "" {
			continue // Skip empty actions
		}
		actions = append(actions, action)

		// Execute to update state for next generation
		_, _ = executor.ExecuteOne(action)
	}

	return actions
}

// selectWeighted selects an action using weighted random selection
func (g *ActionGenerator) selectWeighted(actions []string) string {
	totalWeight := 0.0
	for _, action := range actions {
		weight := g.weights[action]
		if weight <= 0 {
			weight = 1.0 // Default weight
		}
		totalWeight += weight
	}

	target := g.rng.Float64() * totalWeight
	current := 0.0

	for _, action := range actions {
		weight := g.weights[action]
		if weight <= 0 {
			weight = 1.0
		}
		current += weight
		if current >= target {
			return action
		}
	}

	// Fallback to last action
	return actions[len(actions)-1]
}

// discoverActions finds all valid action methods on the controller
func (g *ActionGenerator) discoverActions() {
	controllerType := reflect.TypeOf(g.controller)
	contextType := reflect.TypeOf((*livetemplate.Context)(nil))
	errorType := reflect.TypeOf((*error)(nil)).Elem()

	for i := 0; i < controllerType.NumMethod(); i++ {
		method := controllerType.Method(i)
		methodType := method.Type

		// Check: func(receiver, state, *Context) (state, error)
		if methodType.NumIn() != 3 || methodType.NumOut() != 2 {
			continue
		}

		// First param must match state type
		if methodType.In(1) != g.stateType {
			continue
		}

		// Second param must be *Context
		if methodType.In(2) != contextType {
			continue
		}

		// First output must match state type
		if methodType.Out(0) != g.stateType {
			continue
		}

		// Second output must implement error
		if !methodType.Out(1).Implements(errorType) {
			continue
		}

		// Convert method name to action name (camelCase)
		actionName := toLowerCamelCase(method.Name)
		g.actions = append(g.actions, actionName)
	}
}

// toLowerCamelCase converts PascalCase to camelCase
func toLowerCamelCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// Common data generators

// StringGen creates a random string generator
func StringGen(minLen, maxLen int) DataGen {
	return func(rng *rand.Rand, state interface{}) map[string]interface{} {
		length := minLen + rng.Intn(maxLen-minLen+1)
		return map[string]interface{}{
			"value": randomString(rng, length),
		}
	}
}

// IntGen creates a random int generator
func IntGen(min, max int) DataGen {
	return func(rng *rand.Rand, state interface{}) map[string]interface{} {
		return map[string]interface{}{
			"value": min + rng.Intn(max-min+1),
		}
	}
}

// IndexGen creates a generator that picks a valid index from a slice field
func IndexGen(fieldName string) DataGen {
	return func(rng *rand.Rand, state interface{}) map[string]interface{} {
		v := reflect.ValueOf(state)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		field := v.FieldByName(fieldName)
		if !field.IsValid() || field.Kind() != reflect.Slice {
			return nil
		}

		length := field.Len()
		if length == 0 {
			return nil // No valid index
		}

		return map[string]interface{}{
			"index": rng.Intn(length),
		}
	}
}

// FieldValueGen creates a generator that picks a random value from a slice field
func FieldValueGen(fieldName, keyField string) DataGen {
	return func(rng *rand.Rand, state interface{}) map[string]interface{} {
		v := reflect.ValueOf(state)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		field := v.FieldByName(fieldName)
		if !field.IsValid() || field.Kind() != reflect.Slice {
			return nil
		}

		length := field.Len()
		if length == 0 {
			return nil
		}

		idx := rng.Intn(length)
		elem := field.Index(idx)

		if keyField != "" && elem.Kind() == reflect.Struct {
			keyVal := elem.FieldByName(keyField)
			if keyVal.IsValid() {
				return map[string]interface{}{
					keyField: keyVal.Interface(),
				}
			}
		}

		return map[string]interface{}{
			"index": idx,
		}
	}
}

// randomString generates a random string of the given length
func randomString(rng *rand.Rand, length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rng.Intn(len(charset))]
	}
	return string(b)
}

// FuzzConfig configures fuzz test execution
type FuzzConfig struct {
	MinActions int                 // Minimum actions per sequence
	MaxActions int                 // Maximum actions per sequence
	Weights    map[string]float64  // Action weights
	DataGens   map[string]DataGen  // Data generators
	Skips      map[string]SkipFunc // Skip conditions
	Invariants []Invariant         // Invariants to check
}

// DefaultFuzzConfig returns sensible defaults
func DefaultFuzzConfig() *FuzzConfig {
	return &FuzzConfig{
		MinActions: 5,
		MaxActions: 50,
		Weights:    make(map[string]float64),
		DataGens:   make(map[string]DataGen),
		Skips:      make(map[string]SkipFunc),
		Invariants: DefaultInvariants,
	}
}

// FuzzRunner runs fuzz tests with a given configuration
type FuzzRunner struct {
	controller interface{}
	stateType  reflect.Type
	config     *FuzzConfig
}

// NewFuzzRunner creates a new fuzz runner
func NewFuzzRunner(controller interface{}, stateType reflect.Type, config *FuzzConfig) *FuzzRunner {
	if config == nil {
		config = DefaultFuzzConfig()
	}
	return &FuzzRunner{
		controller: controller,
		stateType:  stateType,
		config:     config,
	}
}

// Run executes a fuzz test with the given seed
func (r *FuzzRunner) Run(seed int64, initialState interface{}) (*FuzzResult, error) {
	gen := NewActionGenerator(r.controller, r.stateType)
	gen.WithSeed(seed)

	// Apply config
	gen.SetWeights(r.config.Weights)
	for action, dataGen := range r.config.DataGens {
		gen.SetDataGen(action, dataGen)
	}
	for action, skip := range r.config.Skips {
		gen.SetSkipFunc(action, skip)
	}

	executor := NewDirectExecutor(r.controller, initialState)
	executor.WithInvariants(r.config.Invariants...)

	// Generate random number of actions
	rng := rand.New(rand.NewSource(seed))
	numActions := r.config.MinActions + rng.Intn(r.config.MaxActions-r.config.MinActions+1)

	// Generate and execute actions
	actions := make([]Action, 0, numActions)
	for i := 0; i < numActions; i++ {
		action := gen.Generate(executor.CurrentState())
		if action.Name == "" {
			continue
		}
		actions = append(actions, action)
	}

	// Reset and run full sequence with invariant checking
	executor.Reset()
	err := executor.RunSequence(actions)

	return &FuzzResult{
		Seed:       seed,
		Actions:    actions,
		FinalState: executor.CurrentState(),
		Error:      err,
	}, err
}

// FuzzResult captures the result of a fuzz test run
type FuzzResult struct {
	Seed       int64
	Actions    []Action
	FinalState interface{}
	Error      error
}

// ReplaySequence returns a string that can be used to reproduce the sequence
func (r *FuzzResult) ReplaySequence() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("// Seed: %d\n", r.Seed))
	sb.WriteString("actions := []seqtest.Action{\n")
	for _, action := range r.Actions {
		if len(action.Data) == 0 {
			sb.WriteString(fmt.Sprintf("    {Name: %q},\n", action.Name))
		} else {
			sb.WriteString(fmt.Sprintf("    {Name: %q, Data: %#v},\n", action.Name, action.Data))
		}
	}
	sb.WriteString("}\n")
	return sb.String()
}

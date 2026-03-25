// Progressive Complexity Demo: Profile Editor (Tier 1 only)
//
// This demo shows LiveTemplate's Tier 1: ZERO lvt-* attributes.
// The form auto-submits to the conventional Submit() method.
// Validation errors display via .lvt.HasError and .lvt.Error helpers.
//
// The Change() method is optional — add it to enable live preview
// updates as the user types (requires Phase 2: inferred bindings).
package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/livetemplate/livetemplate"
)

var validate = validator.New()

// ProfileState is pure session data, cloned per user.
type ProfileState struct {
	DisplayName string
	Email       string
	Bio         string
	Saved       bool
}

// ProfileController is a singleton holding dependencies.
type ProfileController struct {
	Logger *slog.Logger
}

// Submit handles the default form submission (Tier 1: no lvt-submit attribute).
func (c *ProfileController) Submit(state ProfileState, ctx *livetemplate.Context) (ProfileState, error) {
	var input struct {
		DisplayName string `json:"DisplayName" validate:"required,min=2,max=50"`
		Email       string `json:"Email" validate:"required,email"`
		Bio         string `json:"Bio" validate:"max=500"`
	}
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		state.Saved = false
		return state, err
	}

	state.DisplayName = input.DisplayName
	state.Email = input.Email
	state.Bio = input.Bio
	state.Saved = true
	c.Logger.Info("Profile saved",
		slog.String("name", input.DisplayName),
		slog.String("email", input.Email))
	return state, nil
}

func main() {
	controller := &ProfileController{
		Logger: slog.Default(),
	}

	tmpl, err := livetemplate.New("profile")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := tmpl.Parse(profileTemplate); err != nil {
		log.Fatal(err)
	}

	handler := tmpl.Handle(controller, livetemplate.AsState(&ProfileState{
		DisplayName: "Jane Doe",
		Email:       "jane@example.com",
		Bio:         "Go developer and open source enthusiast.",
	}))

	http.Handle("/", handler)

	addr := ":8081"
	fmt.Printf("Profile demo running at http://localhost%s\n", addr)
	fmt.Println("Uses ZERO lvt-* attributes — Tier 1 only!")
	log.Fatal(http.ListenAndServe(addr, nil))
}

const profileTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>Progressive Complexity: Profile Demo</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 600px; margin: 40px auto; padding: 0 20px; }
        h1 { color: #333; }
        .subtitle { color: #666; font-size: 14px; margin-top: -10px; }
        label { display: block; font-weight: 600; margin-top: 16px; margin-bottom: 4px; }
        input, textarea { width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 4px; font-size: 16px; box-sizing: border-box; }
        input.error-input, textarea.error-input { border-color: #dc3545; background: #fff5f5; }
        .error { color: #dc3545; font-size: 13px; margin-top: 4px; }
        button[type="submit"] { margin-top: 20px; padding: 10px 24px; background: #007bff; color: white; border: none; border-radius: 4px; cursor: pointer; font-size: 16px; }
        button[type="submit"]:hover { background: #0056b3; }
        .success { background: #d4edda; color: #155724; padding: 12px; border-radius: 4px; margin-top: 16px; }
        .preview { background: #f8f9fa; padding: 20px; border-radius: 8px; margin-top: 30px; }
        .preview h2 { margin-top: 0; }
    </style>
</head>
<body>
    <h1>Edit Profile</h1>
    <p class="subtitle">Zero <code>lvt-*</code> attributes &mdash; Tier 1 only</p>

    {{if .Saved}}
        <div class="success">Profile saved successfully!</div>
    {{end}}

    <!-- TIER 1: Zero attributes. Form auto-submits to Submit() method. -->
    <form method="POST">
        <label for="name">Display Name</label>
        <input type="text" id="name" name="DisplayName" value="{{.DisplayName}}"
               class="{{if .lvt.HasError "display_name"}}error-input{{end}}">
        {{if .lvt.HasError "display_name"}}
            <div class="error">{{.lvt.Error "display_name"}}</div>
        {{end}}

        <label for="email">Email</label>
        <input type="email" id="email" name="Email" value="{{.Email}}"
               class="{{if .lvt.HasError "email"}}error-input{{end}}">
        {{if .lvt.HasError "email"}}
            <div class="error">{{.lvt.Error "email"}}</div>
        {{end}}

        <label for="bio">Bio</label>
        <textarea id="bio" name="Bio" rows="4">{{.Bio}}</textarea>
        {{if .lvt.HasError "bio"}}
            <div class="error">{{.lvt.Error "bio"}}</div>
        {{end}}

        <button type="submit">Save Profile</button>
    </form>

    <!-- Preview section shows current state -->
    <div class="preview">
        <h2>Preview</h2>
        <p><strong>Name:</strong> {{.DisplayName}}</p>
        <p><strong>Email:</strong> {{.Email}}</p>
        <p><strong>Bio:</strong> {{.Bio}}</p>
    </div>
</body>
</html>`

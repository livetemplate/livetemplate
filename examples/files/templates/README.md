# Template Files

This directory contains example HTML template files used by the file parsing example.

## 📁 Template Files

### `header.html` - Application Header
**Dependencies:** `Title`, `CurrentUser.Name`, `CurrentUser.Role`

A responsive application header with title and user welcome message.

```html
<header class="app-header">
    <div class="header-content">
        <h1 class="app-title">{{.Title}}</h1>
        <div class="user-welcome">
            <span class="greeting">Welcome back, {{.CurrentUser.Name}}!</span>
            <span class="user-role">({{.CurrentUser.Role}})</span>
        </div>
    </div>
</header>
```

### `sidebar.html` - Navigation Sidebar
**Dependencies:** `CurrentUser.Name`, `CurrentUser.Email`, `CurrentUser.Role`, `Stats.UserCount`, `Stats.PostCount`, `Stats.LastUpdate`

A navigation sidebar with user profile information and site statistics.

### `footer.html` - Application Footer
**Dependencies:** `Stats.UserCount`, `Stats.PostCount`

A simple footer displaying site statistics and copyright information.

### `user-profile.html` - User Profile Widget
**Dependencies:** `CurrentUser.ID`, `CurrentUser.Name`, `CurrentUser.Email`, `CurrentUser.Role`

A detailed user profile widget showing all user information.

### `dashboard.html` - Dashboard Widget
**Dependencies:** `Title`, `CurrentUser.Name`, `Stats.UserCount`, `Stats.PostCount`, `Stats.LastUpdate`

A comprehensive dashboard widget with metrics and current user information.

## 🎯 Usage

These templates are loaded by the file parsing example (`../main.go`) in two ways:

1. **Directory Loading** - All `.html` files are loaded automatically
2. **Specific File Loading** - Individual files are loaded with custom names

## 📊 Template Structure

Each template is designed to:
- ✅ Use semantic HTML structure
- 🎯 Have clear dependency patterns
- 🔄 Demonstrate different data binding scenarios
- 📱 Support responsive design classes

## 🔍 Dependency Analysis

The template tracker automatically analyzes these files to determine:
- Which data fields each template depends on
- Which templates need updates when specific data changes
- Optimal re-rendering strategies for performance

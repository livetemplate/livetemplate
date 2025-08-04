# Code Organization Update - NO cmd/ Directory Policy

## Changes Made

### 1. Updated Instructions - General Instructions

Enhanced `.github/instructions/general.instructions.md` with:

- **MANDATORY - NO cmd/ DIRECTORY** rule with clear prohibition
- **MANDATORY - Examples Organization** section with specific guidelines
- **MANDATORY - Debug Code Policy** with clear alternatives

### 2. Updated Instructions - Copilot Instructions

Enhanced `.github/copilot-instructions.md` with:

- New "Code Organization" section as prominent first item
- Same mandatory rules for better visibility during AI interactions
- Clear guidance on where different types of code should go

### 3. Reorganized Existing Code

Removed all content from `cmd/` directory:

#### Actions Taken:

- Removed `cmd/debug/` - debug code (was temporary development code)
- Removed `cmd/debug-html/` - HTML processing debug tools (was temporary)
- Removed `cmd/debug-normalize/` - normalization debug tools (was temporary)
- Removed `cmd/debug-updates/` - updates debug tools (was temporary)
- Removed `cmd/` directory entirely
- Removed debug `main.go` from root directory

All debug/development code was determined to be temporary and safely removed.

### 4. Updated Documentation

- Updated `examples/README.md` to document the new debug examples
- Organized examples into "Core Examples" and "Debug Examples" sections

## New Rules Summary

### ❌ NEVER CREATE:

- `cmd/` directory for any purpose
- Separate debug executables or main.go files
- Example code outside `examples/` directory

### ✅ ALWAYS USE:

- `examples/descriptive-name/` for demo/example code
- `*_test.go` files for temporary debug functions (then delete)
- `examples/e2e/` for end-to-end tests

### Debug Code Policy:

- **Temporary debug code**: Use test functions like `TestDebug_SpecificIssue`
- **Demo/example code**: Place in `examples/` with descriptive subdirectory
- **Permanent debug tools**: Convert to examples or proper test utilities

## Verification

- ✅ `cmd/` directory completely removed
- ✅ All former cmd/ content moved to appropriate examples/ subdirectories
- ✅ Tests still pass
- ✅ Examples still work (verified `examples/debug/main.go`)
- ✅ Documentation updated

## Future Compliance

The enhanced instructions now include:

- **MANDATORY** emphasis to make rules non-optional
- Specific examples of what goes where
- Clear alternatives for different types of code
- Prominence in both general and copilot instructions

This should prevent future creation of `cmd/` directory and ensure proper code organization.

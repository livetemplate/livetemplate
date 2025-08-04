# StateTemplate golangci-lint Configuration

This project intentionally does NOT use a `.golangci.yml` configuration file.

## Reason

The previous `.golangci.yml` configuration caused compatibility issues with golangci-lint v2.3.1 and Go 1.24. Using the default configuration provides better stability and compatibility.

## Default Configuration

The default golangci-lint configuration includes essential linters:

- errcheck, govet, ineffassign, staticcheck, unused

## If You Need to Modify Linter Behavior

1. Test with the current golangci-lint version first
2. Update `scripts/validate-ci.sh` accordingly  
3. Ensure all CI validation passes

## Important

**DO NOT restore `.golangci.yml` without careful testing!**

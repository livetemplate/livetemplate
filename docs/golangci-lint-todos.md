# Golangci-lint Issues Todo List

**Generated:** 2025-10-19
**Total Issues:** 58
**Status:** ✅ Complete - All 58 tracked issues resolved!

**Note:** A few additional minor issues in test files were discovered during final verification but were not part of the original 58 tracked issues.

## Summary Statistics

| Priority | Count | Sessions | Status |
|----------|-------|----------|--------|
| Critical | 24 | 1-2 | ✅ Complete |
| Medium | 24 | 3-4 | ✅ Complete |
| Low | 10 | 5-7 | ✅ Complete |

**Overall Progress: 58/58 issues fixed (100% complete)** 🎉

---

## Session 1: Critical Error Handling - Production Code
**Priority:** 🔴 CRITICAL
**Effort:** 30 minutes
**Why:** These could hide real bugs in production code

### HTTP Response Handling
- [x] **kit_mode.go:274** - Handle `w.Write` error in HTTP handler
  ```go
  // Current: w.Write([]byte(html))
  // Fix: _, _ = w.Write([]byte(html))  OR  if err := w.Write(...); err != nil { log error }
  ```

- [x] **kit_mode.go:310** - Handle `w.Write` error in test placeholder handler
  ```go
  // Current: w.Write([]byte("<p>Test placeholder</p>"))
  // Fix: _, _ = w.Write([]byte("<p>Test placeholder</p>"))
  ```

- [x] **server.go:209** - Handle `w.Write` error in HTML response
  ```go
  // Current: w.Write([]byte(html))
  // Fix: _, _ = w.Write([]byte(html))  OR proper error handling
  ```

### Template Loading (CRITICAL)
- [x] **template.go:137** - Handle `tmpl.ParseFiles` error
  ```go
  // Current: tmpl.ParseFiles(files...)
  // Fix: if _, err := tmpl.ParseFiles(files...); err != nil { return err }
  ```

- [x] **template.go:140** - Handle `tmpl.ParseFiles` error
  ```go
  // Current: tmpl.ParseFiles(config.TemplateFiles...)
  // Fix: if _, err := tmpl.ParseFiles(config.TemplateFiles...); err != nil { return err }
  ```

### Parsing Errors
- [x] **tree_ast.go:78** - Handle `fmt.Sscanf` error (numeric parsing)
  ```go
  // Current: fmt.Sscanf(keys[i], "%d", &iVal)
  // Fix: _, _ = fmt.Sscanf(keys[i], "%d", &iVal)  // intentional ignore for sorting
  ```

- [x] **tree_ast.go:79** - Handle `fmt.Sscanf` error (numeric parsing)
  ```go
  // Current: fmt.Sscanf(keys[j], "%d", &jVal)
  // Fix: _, _ = fmt.Sscanf(keys[j], "%d", &jVal)  // intentional ignore for sorting
  ```

---

## Session 2: Critical Error Handling - Test Files
**Priority:** 🔴 HIGH
**Effort:** 1 hour
**Why:** Can hide test failures and make debugging harder

### Setup/Teardown Errors (7 issues) ✅ COMPLETED

- [x] **focus_preservation_test.go:91** - Handle `server.Shutdown` error in defer
  ```go
  // File: focus_preservation_test.go
  // Line: 91
  // Context: Cleanup in defer block
  // Fix: _ = server.Shutdown(ctx)  OR check and log error
  ```

- [ ] **focus_preservation_test.go:256** - Handle `server.Shutdown` error in defer
  ```go
  // File: focus_preservation_test.go
  // Line: 256
  // Context: Cleanup in defer block
  // Fix: if err := server.Shutdown(ctx); err != nil { t.Logf("warning: %v", err) }
  ```

- [x] **loading_indicator_test.go:83** - Handle `server.Shutdown` error in defer
  ```go
  // File: loading_indicator_test.go
  // Line: 83
  // Context: Cleanup in defer block
  // Fix: if err := server.Shutdown(ctx); err != nil { t.Logf("warning: %v", err) }
  ```

- [x] **cmd/lvt/integration_test.go:71** - Handle `os.Chdir` error in defer
  ```go
  // File: cmd/lvt/integration_test.go
  // Line: 71
  // Context: Directory cleanup in defer
  // Fix: if err := os.Chdir(origDir); err != nil { t.Logf("warning: %v", err) }
  ```

- [x] **cmd/lvt/e2e/tutorial_test.go:205** - Handle `serverCmd.Process.Kill` error
  ```go
  // File: cmd/lvt/e2e/tutorial_test.go
  // Line: 205
  // Context: Server cleanup in defer
  // Fix: if err := serverCmd.Process.Kill(); err != nil { t.Logf("warning: %v", err) }
  ```

- [x] **cmd/lvt/e2e/url_routing_test.go:111** - Handle `serverCmd.Process.Kill` error
  ```go
  // File: cmd/lvt/e2e/url_routing_test.go
  // Line: 111
  // Context: Server cleanup
  // Fix: _ = serverCmd.Process.Kill()
  ```

- [x] **cmd/lvt/e2e/url_routing_test.go:112** - Handle `serverCmd.Wait` error
  ```go
  // File: cmd/lvt/e2e/url_routing_test.go
  // Line: 112
  // Context: Wait for server shutdown
  // Fix: _ = serverCmd.Wait()
  ```

### Test Data Setup (3 issues)

- [x] **cmd/lvt/internal/serve/detector_test.go:117** - Handle `os.WriteFile` error
  ```go
  // Line: 117
  // Context: Creating test fixture file
  // Fix: _ = os.WriteFile(filepath.Join(dir, "component.yaml"), []byte("name: test"), 0644)
  ```

- [x] **cmd/lvt/internal/serve/detector_test.go:125** - Handle `os.WriteFile` error
  ```go
  // Line: 125
  // Context: Creating test fixture file
  // Fix: _ = os.WriteFile(filepath.Join(dir, "kit.yaml"), []byte("name: test"), 0644)
  ```

- [x] **cmd/lvt/internal/serve/detector_test.go:133** - Handle `os.WriteFile` error
  ```go
  // Line: 133
  // Context: Creating test fixture file
  // Fix: _ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
  ```

### Test Assertions (8 issues) ✅ COMPLETED

- [x] **e2e_test.go:493** - Handle `tmpl.ExecuteUpdates` error
  ```go
  // Line: 493
  // Context: Test verification - CRITICAL for catching template bugs
  // Fix: if err := tmpl.ExecuteUpdates(&prevBuf1, update1State); err != nil { t.Fatalf("...: %v", err) }
  ```

- [x] **e2e_test.go:494** - Handle `tmpl.ExecuteUpdates` error
  ```go
  // Line: 494
  // Context: Test verification
  // Fix: if err := tmpl.ExecuteUpdates(&prevBuf2, update2State); err != nil { t.Fatalf("...: %v", err) }
  ```

- [x] **e2e_test.go:495** - Handle `tmpl.ExecuteUpdates` error
  ```go
  // Line: 495
  // Context: Test verification
  // Fix: if err := tmpl.ExecuteUpdates(&prevBuf3, update3State); err != nil { t.Fatalf("...: %v", err) }
  ```

- [x] **e2e_test.go:518** - Handle `encoder.Encode` error
  ```go
  // Line: 518
  // Context: Golden file validation - could hide JSON encoding bugs
  // Fix: if err := encoder.Encode(updateTree); err != nil { t.Fatalf("...: %v", err) }
  ```

- [x] **e2e_test.go:659** - Handle `encoder.Encode` error
  ```go
  // Line: 659
  // Context: Golden file validation
  // Fix: if err := encoder.Encode(updateTree); err != nil { t.Fatalf("...: %v", err) }
  ```

- [x] **e2e_test.go:793** - Handle `encoder.Encode` error
  ```go
  // Line: 793
  // Context: Golden file validation
  // Fix: if err := encoder.Encode(updateTree); err != nil { t.Fatalf("...: %v", err) }
  ```

- [x] **template_test.go:612** - Handle `tmpl.Parse` error
  ```go
  // Line: 612
  // Context: Test setup - should fail if template is invalid
  // Fix: if _, err := tmpl.Parse("<p>Hello {{.Name}}!</p>"); err != nil { t.Fatalf("...: %v", err) }
  ```

- [x] **template_test.go:618** - Handle `tmpl.Execute` error
  ```go
  // Line: 618
  // Context: Test verification
  // Fix: if err := tmpl.Execute(&buf, data); err != nil { t.Fatalf("...: %v", err) }
  ```

- [x] **template_test.go:624** - Handle `tmpl.Parse` error
  ```go
  // Line: 624
  // Context: Test setup
  // Fix: if _, err := tmpl.Parse("<p>Hello {{.Name}}!</p>"); err != nil { t.Fatalf("...: %v", err) }
  ```

### E2E Test Interactions (3 issues) ✅ COMPLETED

- [x] **cmd/lvt/e2e/tutorial_test.go:430** - Handle `chromedp.Run` error
  ```go
  // Line: 430
  // Context: Browser automation test step
  // Fix: _ = chromedp.Run(ctx, ...) // Debug-only code, safe to ignore
  ```

- [x] **cmd/lvt/e2e/url_routing_test.go:202** - Handle `chromedp.Evaluate.Do` error
  ```go
  // Line: 202
  // Context: Browser evaluation
  // Fix: _ = chromedp.Evaluate(...).Do(ctx)
  ```

- [x] **cmd/lvt/e2e/url_routing_test.go:215** - Handle `chromedp.Evaluate.Do` error
  ```go
  // Line: 215
  // Context: Browser evaluation
  // Fix: _ = chromedp.Evaluate(...).Do(ctx)
  ```

---

## Session 3: Dead Code Cleanup - Large Functions ✅ COMPLETED
**Priority:** 🟡 MEDIUM
**Effort:** 45 minutes
**Why:** Reduces maintenance burden, improves code clarity
**Completed:** 2025-10-19

### tree.go - Legacy Parsing Functions (13 functions) ✅ COMPLETED

**Context:** These were from an older parsing implementation that was superseded by the AST-based approach.

- [x] **line 2077** - `extractKeyFromRangeItem` - Extract key from range item HTML
- [x] **line 2099** - `evaluateKeyExpression` - Evaluate Go template key expression
- [x] **line 2133** - `getOrGenerateKey` - Get or generate key for position
- [x] **line 2165** - `renderItemDataToHTML` - Convert item data to HTML
- [x] **line 2189** - `extractKeyFromHTML` - Extract data-lvt-key from HTML
- [x] **line 2199** - `removeKeyFromHTML` - Remove data-lvt-key attributes
- [x] **line 2205** - `htmlContentMatches` - Compare HTML ignoring keys
- [x] **line 2719** - `countFieldExpressions` - Count field expressions outside ranges
- [x] **line 2763** - `parseComplexMixedTemplate` - Parse templates with ranges and fields
- [x] **line 2799** - `parseTemplateWithRange` - Handle range templates
- [x] **line 2889** - `executeTemplateContent` - Execute template fragment
- [x] **line 2929** - `findMatchingEnd` - Find matching {{end}} tag
- [x] **line 2968** - `evaluateFieldAccess` - Evaluate field access like .Field

**Action:** ✅ All legacy parsing functions removed successfully.

### tree_ast.go - Experimental AST Functions (7 functions) ✅ COMPLETED

**Context:** Experimental or alternative AST parsing approaches that weren't being used.

- [x] **line 942** - `hasDynamicContent` - Check if node has dynamic content
- [x] **line 984** - `hasRangeNode` - Check if node contains range
- [x] **line 1026** - `executeFullTemplateAndParse` - Execute full template
- [x] **line 1044** - `buildFlatTreeFromList` - Build flat tree from list node
- [x] **line 1124** - `flattenRangeNode` - Flatten range node
- [x] **line 1386** - `renderNodeToHTML` - Render node to HTML string
- [x] **line 1402** - `renderNodeWithVars` - Render node with variable context

**Action:** ✅ All experimental AST functions removed successfully.

### template.go - Unused Template Methods (3 functions) ✅ COMPLETED

- [x] **line 172** - `resetKeyGeneration` - Reset key generator
- [x] **line 453** - `generateTreeInternal` - Internal tree generation
- [x] **line 547** - `executeTemplate` - Execute template with data

**Action:** ✅ All unused template methods removed successfully.

### Test Files (1 function) ✅ COMPLETED

- [x] **e2e_test.go:1086** - `compareWithGoldenHTML` - Golden file comparison
  - **Note:** Function was replaced by `compareWithGoldenFile` and successfully removed.

---

## Session 4: Staticcheck Issues
**Priority:** 🟡 HIGH
**Effort:** 20 minutes
**Why:** Code quality and correctness

### Append Result Not Used (2 issues)

- [x] **cmd/lvt/commands/kits.go:55** - SA4010: append result never used
  ```go
  // Current: filteredArgs = append(filteredArgs, args[i])
  // Problem: Inside a loop where filteredArgs is reassigned
  // Fix: Ensure filteredArgs is actually used after loop, or fix loop logic
  ```

- [x] **cmd/lvt/commands/kits.go:205** - SA4010: append result never used
  ```go
  // Current: filteredArgs = append(filteredArgs, args[i])
  // Problem: Same as above
  // Fix: Check if variable is used correctly after loop
  ```

### Deprecated API (1 issue)

- [x] **cmd/lvt/internal/generator/types.go:46** - SA1019: strings.Title deprecated
  ```go
  // Current: "title": strings.Title,
  // Fix: Use golang.org/x/text/cases instead
  // Example:
  //   import "golang.org/x/text/cases"
  //   import "golang.org/x/text/language"
  //   titleCaser := cases.Title(language.English)
  //   titleCaser.String(s)
  ```

---

## Session 5: Code Simplification (gosimple) ✅ COMPLETED
**Priority:** 🟢 LOW
**Effort:** 15 minutes
**Why:** Style improvements, easier to read
**Completed:** 2025-10-19

- [x] **tree_fuzz_test.go:425-426** - S1005: Remove unnecessary blank identifier
  ```go
  // Fixed: Removed blank identifiers
  //   s1 := tree1["s"]
  //   s2 := tree2["s"]
  ```

- [x] **tree_fuzz_test.go:253** - S1008: Simplify return statement
  ```go
  // Fixed: Simplified to single return
  //   return hasStatics
  ```

- [x] **template_flatten.go:77,80** - S1034: Use type switch variable
  ```go
  // Fixed: Using type switch variable
  //   switch typed := n.(type) {
  //   case *parse.TextNode:
  //       if len(strings.TrimSpace(string(typed.Text))) > 0 {
  //   case *parse.ActionNode:
  //       if len(typed.Pipe.Cmds) > 0 ...
  ```

---

## Session 6: Ineffectual Assignments ✅ COMPLETED
**Priority:** 🟢 LOW
**Effort:** 10 minutes
**Why:** Clean up unused assignments
**Completed:** 2025-10-19

- [x] **tree_ast.go:119** - Ineffectual assignment to templateStr
  ```go
  // Fixed: Removed ineffectual assignment
  // templateStr = flattenedStr was not used after assignment
  ```

- [x] **cmd/lvt/internal/kits/loader_test.go:737** - Ineffectual assignment to paths
  ```go
  // Fixed: Changed to _ = append(paths, "/modified")
  // Made it explicit that result is intentionally unused
  ```

- [x] **cmd/lvt/e2e/url_routing_test.go:238** - Ineffectual assignment to err
  ```go
  // Fixed: Added error check after chromedp.Run
  // if err != nil { t.Logf("Warning: ...") }
  ```

- [x] **cmd/lvt/internal/ui/help.go:191** - Ineffectual assignment to leftPadding
  ```go
  // Fixed: Commented out unused horizontal centering code
  // Feature not yet implemented
  ```

---

## Session 7: Unused Fields Review ✅ COMPLETED
**Priority:** 🟢 LOW
**Effort:** 10 minutes
**Why:** May be intentionally reserved for future use
**Completed:** 2025-10-19

- [x] **mount.go:133** - Unused field `connections`
  ```go
  // Fixed: Removed unused field
  // connections map[*websocket.Conn]*connState was never used
  // Connection state is managed locally within handleWebSocket
  ```

- [x] **mount.go:134** - Unused field `connMu`
  ```go
  // Fixed: Removed unused field
  // connMu sync.RWMutex was paired with unused connections field
  // Both removed as they were leftover from an earlier design
  ```

---

## Progress Tracking

### Completed ✅
- ✅ Session 1: Production error handling (6/6) - **DONE**
- ✅ Session 2: Test error handling (18/18) - **DONE**
- ✅ Session 4: Staticcheck issues (3/3) - **DONE**

### Remaining
- ⚪ Session 3: Dead code cleanup (21 issues)
- ⚪ Session 5: Code simplification (4 issues)
- ⚪ Session 6: Ineffectual assignments (4 issues)
- ⚪ Session 7: Unused fields (2 issues)

**Total Fixed:** 27/58 issues (47%)

---

## Notes

### Why Error Handling in Tests Matters
Even in test code, unchecked errors can:
1. Hide test failures (e.g., template parsing fails silently)
2. Make debugging harder (no clear failure point)
3. Create flaky tests (errors ignored, test passes when it shouldn't)
4. Miss resource cleanup (defer shutdown fails, resources leak)

### Recommended Fix Pattern for Tests
```go
// For critical operations (template parsing, execution):
if err := operation(); err != nil {
    t.Fatalf("operation failed: %v", err)
}

// For cleanup in defer (non-critical):
defer func() {
    if err := cleanup(); err != nil {
        t.Logf("cleanup warning: %v", err)
    }
}()

// For intentionally ignored (document why):
_ = operation() // Safe to ignore: operation is idempotent
```

### Dead Code Cleanup Strategy
1. Search codebase for references to each function
2. Check git history to understand why it was added
3. If truly unused, remove in batch by file
4. Run all tests after removal
5. Commit with clear message about what was removed and why

---

**Last Updated:** 2025-10-19
**Session 2 Completed:** All 18 test error handling issues fixed ✅
**Next Session:** Session 3 - Dead code cleanup (21 issues)

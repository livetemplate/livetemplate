package discovery

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestWalkErrAction covers the decision table directly, since the states it
// distinguishes are awkward to provoke through a real walk.
func TestWalkErrAction(t *testing.T) {
	dirEntry := fakeDirEntry{name: "gone", dir: true}
	fileEntry := fakeDirEntry{name: "gone.tmpl", dir: false}
	notExist := &fs.PathError{Op: "readdirent", Path: "/root/sub", Err: fs.ErrNotExist}
	denied := &fs.PathError{Op: "open", Path: "/root/sub", Err: fs.ErrPermission}

	tests := []struct {
		name     string
		root     string
		path     string
		d        fs.DirEntry
		err      error
		want     error
		wantSame bool // want the original error back
	}{
		{
			name: "vanished subdirectory is skipped",
			root: "/root", path: "/root/sub", d: dirEntry, err: notExist,
			want: fs.SkipDir,
		},
		{
			// SkipDir here would skip the rest of the containing directory and
			// silently drop sibling templates.
			name: "vanished file is ignored without skipping its siblings",
			root: "/root", path: "/root/sub/x.tmpl", d: fileEntry, err: notExist,
			want: nil,
		},
		{
			// The caller asked to search somewhere that does not exist; reporting
			// "no templates found" would hide that.
			name: "missing root still surfaces",
			root: "/root", path: "/root", d: nil, err: notExist,
			wantSame: true,
		},
		{
			name: "non-ENOENT errors always surface",
			root: "/root", path: "/root/sub", d: dirEntry, err: denied,
			wantSame: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := walkErrAction(tt.root, tt.path, tt.d, tt.err)
			if tt.wantSame {
				if !errors.Is(got, tt.err) {
					t.Errorf("expected the original error back, got %v", got)
				}
				return
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDiscoverTemplateFiles_ToleratesVanishingPaths reproduces issue #502: a
// sibling process removing a directory tree while discovery walks it used to
// abort the whole walk, so livetemplate.New() failed with
// "template auto-discovery failed: readdirent …: no such file or directory" in
// whichever unrelated test happened to be constructing a Template at the time.
//
// The churning directory is deliberately NOT named .uploads. That name is now in
// ignoredTemplateDirs, so using it here would make this pass by never walking
// the tree at all — proving the skip rather than the tolerance. This pins the
// general property: discovery survives any path vanishing underneath it.
func TestDiscoverTemplateFiles_ToleratesVanishingPaths(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "page.tmpl"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("seed template: %v", err)
	}
	churn := filepath.Join(root, "scratch")

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			// Deep enough that a walk is likely to be inside it when it goes.
			deep := filepath.Join(churn, "tmp", "session-1", "field")
			_ = os.MkdirAll(deep, 0o755)
			_ = os.WriteFile(filepath.Join(deep, "part.tmpl"), []byte("x"), 0o644)
			_ = os.RemoveAll(churn)
		}
	}()
	defer func() {
		close(stop)
		wg.Wait()
	}()

	for i := 0; i < 400; i++ {
		files, err := DiscoverTemplateFiles(root, nil)
		if err != nil {
			t.Fatalf("iteration %d: discovery failed while a sibling tree was being removed: %v", i, err)
		}
		// The seeded template must still be found — tolerating the vanished path
		// must not cost us the files that are actually there.
		var found bool
		for _, f := range files {
			if filepath.Base(f) == "page.tmpl" {
				found = true
			}
		}
		if !found {
			t.Fatalf("iteration %d: lost page.tmpl; got %v", i, files)
		}
	}
}

// TestDiscoverTemplateFiles_SkipsUploadsDir pins the cheap half of the fix.
// Upload directories hold uploaded files, never templates, so walking them is
// pure cost plus exposure to the teardown race above.
func TestDiscoverTemplateFiles_SkipsUploadsDir(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "page.tmpl"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("seed template: %v", err)
	}
	uploaded := filepath.Join(root, ".uploads", ".tmp", "session-1")
	if err := os.MkdirAll(uploaded, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// A file that would be picked up as a template if the directory were walked.
	if err := os.WriteFile(filepath.Join(uploaded, "upload.html"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed upload: %v", err)
	}

	files, err := DiscoverTemplateFiles(root, nil)
	if err != nil {
		t.Fatalf("discovery: %v", err)
	}
	for _, f := range files {
		if filepath.Base(f) == "upload.html" {
			t.Errorf("discovery walked .uploads and picked up %s", f)
		}
	}
	if len(files) != 1 || filepath.Base(files[0]) != "page.tmpl" {
		t.Errorf("expected only page.tmpl, got %v", files)
	}
}

// TestDiscoverTemplateFiles_MissingBaseDirStillErrors is the counterweight to
// the tolerance: widening it far enough to swallow this would turn a caller's
// wrong path into a silent empty result.
func TestDiscoverTemplateFiles_MissingBaseDirStillErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := DiscoverTemplateFiles(missing, nil); err == nil {
		t.Error("expected an error for a base directory that does not exist")
	}
}

type fakeDirEntry struct {
	name string
	dir  bool
}

func (f fakeDirEntry) Name() string               { return f.name }
func (f fakeDirEntry) IsDir() bool                { return f.dir }
func (f fakeDirEntry) Type() fs.FileMode          { return 0 }
func (f fakeDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

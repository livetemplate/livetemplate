package upload

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/livetemplate/livetemplate/internal/uploadtypes"
)

const (
	// MaxMemory is the maximum amount of memory to use when parsing multipart forms.
	// Files larger than this will be stored on disk.
	MaxMemory = 32 << 20 // 32 MB
)

// ParseMultipartUpload parses multipart form data and creates upload entries.
// Files are streamed to temporary storage managed by tempFileManager.
// Returns entries for all files found, with validation errors marked in entries.
func ParseMultipartUpload(
	r *http.Request,
	uploadName string,
	config uploadtypes.UploadConfig,
	sessionID string,
	tempFileManager *TempFileManager,
) ([]*uploadtypes.UploadEntry, error) {
	// Parse multipart form (if not already parsed)
	if r.MultipartForm == nil {
		if err := r.ParseMultipartForm(MaxMemory); err != nil {
			return nil, fmt.Errorf("failed to parse multipart form: %w", err)
		}
	}

	// Get files from the upload field
	files := r.MultipartForm.File[uploadName]
	if len(files) == 0 {
		return nil, fmt.Errorf("no files found for upload field %q", uploadName)
	}

	// Validate count before processing
	if err := ValidateCount(len(files), config); err != nil {
		return nil, err
	}

	var entries []*uploadtypes.UploadEntry

	// Process each file
	for _, fileHeader := range files {
		entry, err := processMultipartFile(fileHeader, uploadName, config, sessionID, tempFileManager)
		if err != nil {
			// Critical error (not validation error) - abort entire upload
			// Clean up any temp files we've created
			for _, e := range entries {
				if e.TempPath != "" {
					tempFileManager.RemoveFile(e.ID)
				}
			}
			return nil, fmt.Errorf("failed to process file %q: %w", fileHeader.Filename, err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// processMultipartFile processes a single file from multipart form data.
func processMultipartFile(
	fileHeader *multipart.FileHeader,
	uploadName string,
	config uploadtypes.UploadConfig,
	sessionID string,
	tempFileManager *TempFileManager,
) (*uploadtypes.UploadEntry, error) {
	// Generate entry ID
	entryID := GenerateEntryID()

	// Create entry
	entry := &uploadtypes.UploadEntry{
		ID:         entryID,
		ClientName: fileHeader.Filename,
		ClientType: fileHeader.Header.Get("Content-Type"),
		ClientSize: fileHeader.Size,
		Progress:   0,
		Done:       false,
	}

	// Validate entry
	if err := ValidateEntry(entry, config); err != nil {
		entry.Valid = false
		entry.Error = err.Error()
		// Return entry with validation error, but don't fail
		return entry, nil
	}

	// Open the uploaded file
	src, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	// Create temp file
	tempPath, err := tempFileManager.CreateTempFile(sessionID, uploadName, entryID)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// Open temp file for writing
	dst, err := tempFileManager.OpenForWriting(tempPath)
	if err != nil {
		tempFileManager.RemoveFile(entryID)
		return nil, fmt.Errorf("failed to open temp file for writing: %w", err)
	}
	defer dst.Close()

	// Stream file to disk with size limit
	maxSize := config.MaxFileSize
	if maxSize == 0 {
		maxSize = 100 * 1024 * 1024 // Default 100MB limit
	}

	limitedReader := io.LimitReader(src, maxSize+1) // +1 to detect oversized files
	written, err := io.Copy(dst, limitedReader)
	if err != nil {
		tempFileManager.RemoveFile(entryID)
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Check if file exceeded limit
	if written > maxSize {
		tempFileManager.RemoveFile(entryID)
		entry.Valid = false
		entry.Error = fmt.Sprintf("file size %d bytes exceeds maximum %d bytes", written, maxSize)
		return entry, nil
	}

	// Mark as valid and done
	entry.Valid = true
	entry.Done = true
	entry.Progress = 100
	entry.TempPath = tempPath

	return entry, nil
}

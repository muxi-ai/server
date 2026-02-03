package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/muxi-ai/server/pkg/formation"
	"github.com/muxi-ai/server/pkg/registry"
)

// DraftRequest represents the request body for draft file operations
type DraftRequest struct {
	Action   string `json:"action"`             // init, list, read, write, delete, deploy, discard
	Mode     string `json:"mode,omitempty"`     // For init: "new" or "clone"
	Path     string `json:"path,omitempty"`     // For file operations
	Content  string `json:"content,omitempty"`  // For write
	Encoding string `json:"encoding,omitempty"` // "utf-8" (default) or "base64"
}

// DraftMeta represents the .draft-meta.json file
type DraftMeta struct {
	BaseVersion string    `json:"base_version,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// DraftStatus represents draft status in responses
type DraftStatus struct {
	Exists      bool       `json:"exists"`
	BaseVersion string     `json:"base_version,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
}

// LiveStatus represents live formation status in responses
type LiveStatus struct {
	Version string `json:"version,omitempty"`
	Status  string `json:"status,omitempty"`
}

// DraftResponse represents the response for draft operations
type DraftResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
	Draft   DraftStatus `json:"draft"`
	Live    LiveStatus  `json:"live"`
}

// FileEntry represents a file or directory in list response
type FileEntry struct {
	Name string `json:"name"`
	Type string `json:"type"` // "file" or "dir"
	Size int64  `json:"size,omitempty"`
}

// HandleDraft handles POST /rpc/formations/{id}/draft/files
func (s *Server) HandleDraft(w http.ResponseWriter, r *http.Request) {
	formationID := mux.Vars(r)["id"]

	// Validate formation ID format
	if err := registry.ValidateFormationID(formationID); err != nil {
		s.respondDraftError(w, http.StatusBadRequest, "InvalidFormationID", err.Error(), formationID)
		return
	}

	// Parse request body
	var req DraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondDraftError(w, http.StatusBadRequest, "InvalidJSON", "Invalid JSON request body", formationID)
		return
	}

	// Route to appropriate handler
	switch req.Action {
	case "init":
		s.draftInit(w, formationID, req)
	case "list":
		s.draftList(w, formationID, req)
	case "read":
		s.draftRead(w, formationID, req)
	case "write":
		s.draftWrite(w, formationID, req)
	case "delete":
		s.draftDelete(w, formationID, req)
	case "deploy":
		s.draftDeploy(w, r, formationID, req)
	case "discard":
		s.draftDiscard(w, formationID, req)
	default:
		s.respondDraftError(w, http.StatusBadRequest, "InvalidAction",
			fmt.Sprintf("Unknown action: %s. Valid actions: init, list, read, write, delete, deploy, discard", req.Action),
			formationID)
	}
}

// draftInit initializes a new draft
func (s *Server) draftInit(w http.ResponseWriter, formationID string, req DraftRequest) {
	muxiDir, _ := getMuxiDir()
	formationBaseDir := filepath.Join(muxiDir, "formations", formationID)
	draftDir := filepath.Join(formationBaseDir, "draft")
	currentDir := filepath.Join(formationBaseDir, "current")

	// Check if draft already exists
	if _, err := os.Stat(draftDir); err == nil {
		s.respondDraftError(w, http.StatusConflict, "DraftAlreadyExists",
			"A draft already exists for this formation. Use 'discard' to remove it first.", formationID)
		return
	}

	switch req.Mode {
	case "new":
		// Create draft directory with minimal template
		if err := os.MkdirAll(draftDir, 0755); err != nil {
			s.respondDraftError(w, http.StatusInternalServerError, "CreateError",
				"Failed to create draft directory", formationID)
			return
		}

		// Create minimal formation.afs template
		template := fmt.Sprintf(`id: %s
version: 1.0.0
name: %s
`, formationID, formationID)

		if err := os.WriteFile(filepath.Join(draftDir, "formation.afs"), []byte(template), 0644); err != nil {
			os.RemoveAll(draftDir)
			s.respondDraftError(w, http.StatusInternalServerError, "WriteError",
				"Failed to create formation.afs template", formationID)
			return
		}

		// Create draft metadata
		meta := DraftMeta{CreatedAt: time.Now()}
		s.saveDraftMeta(draftDir, meta)

		s.logger.Info().Str("id", formationID).Msg("Created new draft")

	case "clone":
		// Check if current exists
		if _, err := os.Stat(currentDir); os.IsNotExist(err) {
			s.respondDraftError(w, http.StatusNotFound, "LiveNotFound",
				"No live version exists to clone. Use mode 'new' to create a new formation.", formationID)
			return
		}

		// Copy current to draft
		if err := copyDir(currentDir, draftDir); err != nil {
			os.RemoveAll(draftDir)
			s.respondDraftError(w, http.StatusInternalServerError, "CopyError",
				fmt.Sprintf("Failed to clone live version: %v", err), formationID)
			return
		}

		// Get current version for metadata
		var baseVersion string
		if f, err := s.registry.Get(formationID); err == nil {
			baseVersion = f.Version
		}

		// Create draft metadata
		meta := DraftMeta{
			BaseVersion: baseVersion,
			CreatedAt:   time.Now(),
		}
		s.saveDraftMeta(draftDir, meta)

		s.logger.Info().Str("id", formationID).Str("base_version", baseVersion).Msg("Cloned live to draft")

	default:
		s.respondDraftError(w, http.StatusBadRequest, "InvalidMode",
			"Invalid mode. Use 'new' to create empty draft or 'clone' to copy live version.", formationID)
		return
	}

	s.respondDraftSuccess(w, http.StatusOK, map[string]string{"action": "init", "mode": req.Mode}, formationID)
}

// draftList lists files in the draft directory
func (s *Server) draftList(w http.ResponseWriter, formationID string, req DraftRequest) {
	muxiDir, _ := getMuxiDir()
	draftDir := filepath.Join(muxiDir, "formations", formationID, "draft")

	// Check draft exists
	if _, err := os.Stat(draftDir); os.IsNotExist(err) {
		s.respondDraftError(w, http.StatusNotFound, "DraftNotFound",
			"No draft exists for this formation. Use 'init' to create one.", formationID)
		return
	}

	// Sanitize and validate path
	reqPath := req.Path
	if reqPath == "" || reqPath == "/" {
		reqPath = "."
	}
	reqPath = filepath.Clean(reqPath)

	// Prevent path traversal
	if strings.HasPrefix(reqPath, "..") || strings.Contains(reqPath, "/../") {
		s.respondDraftError(w, http.StatusBadRequest, "InvalidPath",
			"Path traversal not allowed", formationID)
		return
	}

	targetDir := filepath.Join(draftDir, reqPath)

	// Ensure target is within draft directory
	if !strings.HasPrefix(targetDir, draftDir) {
		s.respondDraftError(w, http.StatusBadRequest, "InvalidPath",
			"Path must be within draft directory", formationID)
		return
	}

	// Read directory
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			s.respondDraftError(w, http.StatusNotFound, "FileNotFound",
				fmt.Sprintf("Directory not found: %s", reqPath), formationID)
		} else {
			s.respondDraftError(w, http.StatusInternalServerError, "ReadError",
				fmt.Sprintf("Failed to read directory: %v", err), formationID)
		}
		return
	}

	// Build response
	var files []FileEntry
	for _, entry := range entries {
		// Skip .draft-meta.json
		if entry.Name() == ".draft-meta.json" {
			continue
		}

		fe := FileEntry{
			Name: entry.Name(),
		}
		if entry.IsDir() {
			fe.Type = "dir"
			fe.Name += "/"
		} else {
			fe.Type = "file"
			if info, err := entry.Info(); err == nil {
				fe.Size = info.Size()
			}
		}
		files = append(files, fe)
	}

	s.respondDraftSuccess(w, http.StatusOK, map[string]interface{}{
		"path":    reqPath,
		"entries": files,
	}, formationID)
}

// draftRead reads a file from the draft
func (s *Server) draftRead(w http.ResponseWriter, formationID string, req DraftRequest) {
	muxiDir, _ := getMuxiDir()
	draftDir := filepath.Join(muxiDir, "formations", formationID, "draft")

	// Check draft exists
	if _, err := os.Stat(draftDir); os.IsNotExist(err) {
		s.respondDraftError(w, http.StatusNotFound, "DraftNotFound",
			"No draft exists for this formation.", formationID)
		return
	}

	// Validate path
	if req.Path == "" {
		s.respondDraftError(w, http.StatusBadRequest, "InvalidPath", "Path is required", formationID)
		return
	}

	reqPath := filepath.Clean(req.Path)
	if strings.HasPrefix(reqPath, "..") {
		s.respondDraftError(w, http.StatusBadRequest, "InvalidPath",
			"Path traversal not allowed", formationID)
		return
	}

	targetFile := filepath.Join(draftDir, reqPath)
	if !strings.HasPrefix(targetFile, draftDir) {
		s.respondDraftError(w, http.StatusBadRequest, "InvalidPath",
			"Path must be within draft directory", formationID)
		return
	}

	// Read file
	content, err := os.ReadFile(targetFile)
	if err != nil {
		if os.IsNotExist(err) {
			s.respondDraftError(w, http.StatusNotFound, "FileNotFound",
				fmt.Sprintf("File not found: %s", reqPath), formationID)
		} else {
			s.respondDraftError(w, http.StatusInternalServerError, "ReadError",
				fmt.Sprintf("Failed to read file: %v", err), formationID)
		}
		return
	}

	// Determine encoding
	encoding := req.Encoding
	if encoding == "" {
		encoding = "utf-8"
	}

	var contentStr string
	if encoding == "base64" {
		contentStr = base64.StdEncoding.EncodeToString(content)
	} else {
		contentStr = string(content)
	}

	s.respondDraftSuccess(w, http.StatusOK, map[string]interface{}{
		"path":     reqPath,
		"content":  contentStr,
		"encoding": encoding,
	}, formationID)
}

// draftWrite writes a file to the draft
func (s *Server) draftWrite(w http.ResponseWriter, formationID string, req DraftRequest) {
	muxiDir, _ := getMuxiDir()
	draftDir := filepath.Join(muxiDir, "formations", formationID, "draft")

	// Check draft exists
	if _, err := os.Stat(draftDir); os.IsNotExist(err) {
		s.respondDraftError(w, http.StatusNotFound, "DraftNotFound",
			"No draft exists for this formation. Use 'init' to create one.", formationID)
		return
	}

	// Validate path
	if req.Path == "" {
		s.respondDraftError(w, http.StatusBadRequest, "InvalidPath", "Path is required", formationID)
		return
	}

	reqPath := filepath.Clean(req.Path)
	if strings.HasPrefix(reqPath, "..") {
		s.respondDraftError(w, http.StatusBadRequest, "InvalidPath",
			"Path traversal not allowed", formationID)
		return
	}

	targetFile := filepath.Join(draftDir, reqPath)
	if !strings.HasPrefix(targetFile, draftDir) {
		s.respondDraftError(w, http.StatusBadRequest, "InvalidPath",
			"Path must be within draft directory", formationID)
		return
	}

	// Decode content
	var content []byte
	if req.Encoding == "base64" {
		var err error
		content, err = base64.StdEncoding.DecodeString(req.Content)
		if err != nil {
			s.respondDraftError(w, http.StatusBadRequest, "DecodeError",
				"Invalid base64 content", formationID)
			return
		}
	} else {
		content = []byte(req.Content)
	}

	// Create parent directories if needed
	parentDir := filepath.Dir(targetFile)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		s.respondDraftError(w, http.StatusInternalServerError, "CreateError",
			"Failed to create parent directories", formationID)
		return
	}

	// Write file
	if err := os.WriteFile(targetFile, content, 0644); err != nil {
		s.respondDraftError(w, http.StatusInternalServerError, "WriteError",
			fmt.Sprintf("Failed to write file: %v", err), formationID)
		return
	}

	s.logger.Debug().Str("id", formationID).Str("path", reqPath).Int("size", len(content)).Msg("Wrote draft file")

	s.respondDraftSuccess(w, http.StatusOK, map[string]interface{}{
		"path":    reqPath,
		"size":    len(content),
		"written": true,
	}, formationID)
}

// draftDelete deletes a file or directory from the draft
func (s *Server) draftDelete(w http.ResponseWriter, formationID string, req DraftRequest) {
	muxiDir, _ := getMuxiDir()
	draftDir := filepath.Join(muxiDir, "formations", formationID, "draft")

	// Check draft exists
	if _, err := os.Stat(draftDir); os.IsNotExist(err) {
		s.respondDraftError(w, http.StatusNotFound, "DraftNotFound",
			"No draft exists for this formation.", formationID)
		return
	}

	// Validate path
	if req.Path == "" {
		s.respondDraftError(w, http.StatusBadRequest, "InvalidPath", "Path is required", formationID)
		return
	}

	reqPath := filepath.Clean(req.Path)
	if strings.HasPrefix(reqPath, "..") || reqPath == "." || reqPath == "/" {
		s.respondDraftError(w, http.StatusBadRequest, "InvalidPath",
			"Invalid path", formationID)
		return
	}

	targetPath := filepath.Join(draftDir, reqPath)
	if !strings.HasPrefix(targetPath, draftDir+string(os.PathSeparator)) {
		s.respondDraftError(w, http.StatusBadRequest, "InvalidPath",
			"Path must be within draft directory", formationID)
		return
	}

	// Check if exists
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		s.respondDraftError(w, http.StatusNotFound, "FileNotFound",
			fmt.Sprintf("Path not found: %s", reqPath), formationID)
		return
	}

	// Delete
	if err := os.RemoveAll(targetPath); err != nil {
		s.respondDraftError(w, http.StatusInternalServerError, "DeleteError",
			fmt.Sprintf("Failed to delete: %v", err), formationID)
		return
	}

	s.logger.Debug().Str("id", formationID).Str("path", reqPath).Msg("Deleted draft file/directory")

	s.respondDraftSuccess(w, http.StatusOK, map[string]interface{}{
		"path":    reqPath,
		"deleted": true,
	}, formationID)
}

// draftDeploy deploys the draft to live
func (s *Server) draftDeploy(w http.ResponseWriter, r *http.Request, formationID string, req DraftRequest) {
	muxiDir, _ := getMuxiDir()
	formationBaseDir := filepath.Join(muxiDir, "formations", formationID)
	draftDir := filepath.Join(formationBaseDir, "draft")
	currentDir := filepath.Join(formationBaseDir, "current")

	// Check draft exists
	if _, err := os.Stat(draftDir); os.IsNotExist(err) {
		s.respondDraftError(w, http.StatusNotFound, "DraftNotFound",
			"No draft exists for this formation.", formationID)
		return
	}

	// Parse formation config to get version
	formationConfigPath, err := formation.FindFormationFile(draftDir)
	if err != nil {
		s.respondDraftError(w, http.StatusBadRequest, "ParseError",
			fmt.Sprintf("Failed to find formation config in draft: %v", err), formationID)
		return
	}
	formationConfig, err := formation.ParseFormation(formationConfigPath)
	if err != nil {
		s.respondDraftError(w, http.StatusBadRequest, "ParseError",
			fmt.Sprintf("Failed to parse formation config: %v", err), formationID)
		return
	}

	version := formationConfig.Version
	if version == "" {
		version = "1.0.0"
	}

	// Check if client wants SSE streaming
	acceptHeader := r.Header.Get("Accept")
	wantsSSE := strings.Contains(acceptHeader, "text/event-stream")

	// Initialize progress emitter
	var progress *ProgressEmitter
	if wantsSSE {
		sse, ok := NewSSEWriter(w)
		if ok {
			sse.Init()
			progress = NewProgressEmitter(sse)
		} else {
			progress = NewProgressEmitter(nil)
		}
	} else {
		progress = NewProgressEmitter(nil)
	}

	// Determine if this is a new deploy or update
	_, currentErr := os.Stat(currentDir)
	isNew := os.IsNotExist(currentErr)

	// Also check if formation is registered (could have current/ but not be running)
	_, registryErr := s.registry.Get(formationID)
	isRegistered := registryErr == nil

	s.logger.Info().
		Str("id", formationID).
		Str("version", version).
		Bool("is_new", isNew).
		Bool("is_registered", isRegistered).
		Msg("Deploying draft")

	if isNew || !isRegistered {
		// New formation deploy
		progress.Emit(ProgressEvent{
			Stage:   StageValidating,
			Message: "Deploying new formation from draft...",
		})

		// Use the shared deploy logic - it will MOVE draftDir to current/
		s.deployNewFromDirectory(w, formationID, version, draftDir, nil, wantsSSE, progress)
	} else {
		// Mark formation as deploying
		if err := s.registry.SetDeploying(formationID, true); err != nil {
			s.respondDraftError(w, http.StatusConflict, "DeployConflict",
				"Formation is already being updated", formationID)
			return
		}
		defer s.registry.SetDeploying(formationID, false)

		progress.Emit(ProgressEvent{
			Stage:   StageValidating,
			Message: "Updating formation from draft...",
		})

		// Use the shared update logic - it will MOVE draftDir to staging/
		s.updateFromDirectory(w, formationID, version, draftDir, nil, wantsSSE, progress)
	}
}

// draftDiscard discards the draft
func (s *Server) draftDiscard(w http.ResponseWriter, formationID string, req DraftRequest) {
	muxiDir, _ := getMuxiDir()
	draftDir := filepath.Join(muxiDir, "formations", formationID, "draft")

	// Check draft exists
	if _, err := os.Stat(draftDir); os.IsNotExist(err) {
		s.respondDraftError(w, http.StatusNotFound, "DraftNotFound",
			"No draft exists for this formation.", formationID)
		return
	}

	// Remove draft directory
	if err := os.RemoveAll(draftDir); err != nil {
		s.respondDraftError(w, http.StatusInternalServerError, "DeleteError",
			fmt.Sprintf("Failed to discard draft: %v", err), formationID)
		return
	}

	s.logger.Info().Str("id", formationID).Msg("Discarded draft")

	s.respondDraftSuccess(w, http.StatusOK, map[string]interface{}{
		"discarded": true,
	}, formationID)
}

// Helper functions

func (s *Server) respondDraftSuccess(w http.ResponseWriter, status int, data interface{}, formationID string) {
	resp := DraftResponse{
		Success: true,
		Data:    data,
		Draft:   s.getDraftStatus(formationID),
		Live:    s.getLiveStatus(formationID),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) respondDraftError(w http.ResponseWriter, status int, errType, message, formationID string) {
	resp := DraftResponse{
		Success: false,
		Error:   errType,
		Message: message,
		Draft:   s.getDraftStatus(formationID),
		Live:    s.getLiveStatus(formationID),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) getDraftStatus(formationID string) DraftStatus {
	muxiDir, _ := getMuxiDir()
	draftDir := filepath.Join(muxiDir, "formations", formationID, "draft")

	status := DraftStatus{Exists: false}

	if _, err := os.Stat(draftDir); err == nil {
		status.Exists = true

		// Read metadata
		metaPath := filepath.Join(draftDir, ".draft-meta.json")
		if data, err := os.ReadFile(metaPath); err == nil {
			var meta DraftMeta
			if json.Unmarshal(data, &meta) == nil {
				status.BaseVersion = meta.BaseVersion
				status.CreatedAt = &meta.CreatedAt
			}
		}
	}

	return status
}

func (s *Server) getLiveStatus(formationID string) LiveStatus {
	status := LiveStatus{}

	if f, err := s.registry.Get(formationID); err == nil {
		status.Version = f.Version
		status.Status = f.Status
	}

	return status
}

func (s *Server) saveDraftMeta(draftDir string, meta DraftMeta) error {
	metaPath := filepath.Join(draftDir, ".draft-meta.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, data, 0644)
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		// Copy file
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

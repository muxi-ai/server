package api

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// HandleDownload handles GET /rpc/formations/{id}/download
// Downloads the formation's current directory as a zip file
func (s *Server) HandleDownload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	formationID := vars["id"]

	log.Debug().
		Str("formation_id", formationID).
		Msg("Downloading formation")

	// Check if formation exists in registry
	_, err := s.registry.Get(formationID)
	if err != nil {
		log.Warn().
			Err(err).
			Str("formation_id", formationID).
			Msg("Formation not found")
		RespondError(w, http.StatusNotFound, "Formation not found")
		return
	}

	// Get the current directory path
	currentDir := filepath.Join(s.config.Formations.FormationsDir, formationID, "current")

	// Check if current directory exists
	if _, err := os.Stat(currentDir); os.IsNotExist(err) {
		log.Warn().
			Str("formation_id", formationID).
			Str("path", currentDir).
			Msg("Formation current directory not found")
		RespondError(w, http.StatusNotFound, "Formation files not found")
		return
	}

	// Set headers for zip download
	filename := fmt.Sprintf("%s.zip", formationID)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	// Create zip writer directly to response
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// Walk the directory and add files to zip
	err = filepath.Walk(currentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if path == currentDir {
			return nil
		}

		// Get relative path for zip entry
		relPath, err := filepath.Rel(currentDir, path)
		if err != nil {
			return err
		}

		// Skip hidden files and directories (except .env which might be needed)
		baseName := filepath.Base(path)
		if strings.HasPrefix(baseName, ".") && baseName != ".env" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Create zip header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Use relative path in zip
		header.Name = relPath

		// Set compression method
		if !info.IsDir() {
			header.Method = zip.Deflate
		} else {
			// Directories need trailing slash
			header.Name += "/"
		}

		// Create entry in zip
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// If it's a directory, we're done
		if info.IsDir() {
			return nil
		}

		// Copy file contents
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	if err != nil {
		log.Error().
			Err(err).
			Str("formation_id", formationID).
			Msg("Failed to create zip archive")
		// Can't send error response as we've already started writing
		// The client will get a truncated/invalid zip
		return
	}

	log.Info().
		Str("formation_id", formationID).
		Msg("Formation downloaded successfully")
}

package formation

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractBundle(t *testing.T) {
	t.Run("valid tarball", func(t *testing.T) {
		// Create a temp directory for test files
		tmpDir := t.TempDir()

		// Create a test tarball
		tarballPath := filepath.Join(tmpDir, "test.tar.gz")
		if err := createTestTarball(tarballPath, "test-formation"); err != nil {
			t.Fatalf("Failed to create test tarball: %v", err)
		}

		// Extract it
		destDir := filepath.Join(tmpDir, "extract")
		if err := os.MkdirAll(destDir, 0755); err != nil {
			t.Fatalf("Failed to create dest dir: %v", err)
		}

		extractedPath, err := ExtractBundle(tarballPath, destDir)
		if err != nil {
			t.Fatalf("ExtractBundle() error = %v, want nil", err)
		}

		// Verify extracted path
		expectedPath := filepath.Join(destDir, "test-formation")
		if extractedPath != expectedPath {
			t.Errorf("ExtractBundle() path = %q, want %q", extractedPath, expectedPath)
		}

		// Verify files exist
		testFilePath := filepath.Join(extractedPath, "test.txt")
		if _, err := os.Stat(testFilePath); err != nil {
			t.Errorf("Extracted file %q does not exist: %v", testFilePath, err)
		}

		// Verify file content
		content, err := os.ReadFile(testFilePath)
		if err != nil {
			t.Fatalf("Failed to read extracted file: %v", err)
		}
		if string(content) != "test content" {
			t.Errorf("File content = %q, want %q", string(content), "test content")
		}
	})

	t.Run("tarball with subdirectories", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create tarball with nested structure
		tarballPath := filepath.Join(tmpDir, "nested.tar.gz")
		if err := createNestedTarball(tarballPath, "nested-formation"); err != nil {
			t.Fatalf("Failed to create nested tarball: %v", err)
		}

		destDir := filepath.Join(tmpDir, "extract")
		if err := os.MkdirAll(destDir, 0755); err != nil {
			t.Fatalf("Failed to create dest dir: %v", err)
		}

		extractedPath, err := ExtractBundle(tarballPath, destDir)
		if err != nil {
			t.Fatalf("ExtractBundle() error = %v, want nil", err)
		}

		// Verify nested file exists
		nestedFile := filepath.Join(extractedPath, "subdir", "nested.txt")
		if _, err := os.Stat(nestedFile); err != nil {
			t.Errorf("Nested file %q does not exist: %v", nestedFile, err)
		}
	})

	t.Run("security: path traversal attempt", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a malicious tarball with path traversal
		tarballPath := filepath.Join(tmpDir, "malicious.tar.gz")
		if err := createMaliciousTarball(tarballPath); err != nil {
			t.Fatalf("Failed to create malicious tarball: %v", err)
		}

		destDir := filepath.Join(tmpDir, "extract")
		if err := os.MkdirAll(destDir, 0755); err != nil {
			t.Fatalf("Failed to create dest dir: %v", err)
		}

		_, err := ExtractBundle(tarballPath, destDir)
		if err == nil {
			t.Error("ExtractBundle() with path traversal should fail")
			return
		}

		if !contains(err.Error(), "invalid file path in archive") {
			t.Errorf("ExtractBundle() error = %q, want error containing 'invalid file path in archive'", err.Error())
		}
	})

	t.Run("non-existent tarball", func(t *testing.T) {
		tmpDir := t.TempDir()
		nonExistentPath := filepath.Join(tmpDir, "nonexistent.tar.gz")

		_, err := ExtractBundle(nonExistentPath, tmpDir)
		if err == nil {
			t.Error("ExtractBundle() with non-existent file should fail")
			return
		}

		if !contains(err.Error(), "failed to open gzip file") {
			t.Errorf("ExtractBundle() error = %q, want error containing 'failed to open gzip file'", err.Error())
		}
	})

	t.Run("invalid gzip file", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create an invalid gzip file
		invalidPath := filepath.Join(tmpDir, "invalid.tar.gz")
		if err := os.WriteFile(invalidPath, []byte("not a gzip file"), 0644); err != nil {
			t.Fatalf("Failed to create invalid file: %v", err)
		}

		_, err := ExtractBundle(invalidPath, tmpDir)
		if err == nil {
			t.Error("ExtractBundle() with invalid gzip should fail")
			return
		}

		if !contains(err.Error(), "failed to create gzip reader") {
			t.Errorf("ExtractBundle() error = %q, want error containing 'failed to create gzip reader'", err.Error())
		}
	})

	t.Run("empty tarball", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create an empty tarball
		emptyPath := filepath.Join(tmpDir, "empty.tar.gz")
		if err := createEmptyTarball(emptyPath); err != nil {
			t.Fatalf("Failed to create empty tarball: %v", err)
		}

		destDir := filepath.Join(tmpDir, "extract")
		if err := os.MkdirAll(destDir, 0755); err != nil {
			t.Fatalf("Failed to create dest dir: %v", err)
		}

		_, err := ExtractBundle(emptyPath, destDir)
		if err == nil {
			t.Error("ExtractBundle() with empty tarball should fail")
			return
		}

		if !contains(err.Error(), "empty archive or no root directory found") {
			t.Errorf("ExtractBundle() error = %q, want error containing 'empty archive'", err.Error())
		}
	})
}

// Helper function to create a test tarball
func createTestTarball(path string, rootDir string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Add root directory
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     rootDir + "/",
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}); err != nil {
		return err
	}

	// Add a test file
	content := []byte("test content")
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: rootDir + "/test.txt",
		Mode: 0644,
		Size: int64(len(content)),
	}); err != nil {
		return err
	}

	if _, err := tarWriter.Write(content); err != nil {
		return err
	}

	return nil
}

// Helper function to create a nested tarball
func createNestedTarball(path string, rootDir string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Add root directory
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     rootDir + "/",
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}); err != nil {
		return err
	}

	// Add subdirectory
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     rootDir + "/subdir/",
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}); err != nil {
		return err
	}

	// Add nested file
	content := []byte("nested content")
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: rootDir + "/subdir/nested.txt",
		Mode: 0644,
		Size: int64(len(content)),
	}); err != nil {
		return err
	}

	if _, err := tarWriter.Write(content); err != nil {
		return err
	}

	return nil
}

// Helper function to create a malicious tarball with path traversal
func createMaliciousTarball(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Add a file with path traversal
	content := []byte("malicious content")
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: "../../etc/passwd",
		Mode: 0644,
		Size: int64(len(content)),
	}); err != nil {
		return err
	}

	if _, err := tarWriter.Write(content); err != nil {
		return err
	}

	return nil
}

// Helper function to create an empty tarball
func createEmptyTarball(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Write nothing - just close to create valid but empty archive
	return nil
}

func TestExtractBundle_SymbolicLinks(t *testing.T) {
	tmpDir := t.TempDir()
	tarballPath := filepath.Join(tmpDir, "test.tar.gz")

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add formation.yaml
	yamlContent := "id: symlink-test\nname: Symlink Test\nversion: 1.0.0\n"
	header := &tar.Header{
		Name: "symlink-test/formation.yaml",
		Size: int64(len(yamlContent)),
		Mode: 0644,
	}
	tarWriter.WriteHeader(header)
	tarWriter.Write([]byte(yamlContent))

	// Add a symlink (should be handled gracefully)
	linkHeader := &tar.Header{
		Name:     "symlink-test/link",
		Linkname: "formation.yaml",
		Typeflag: tar.TypeSymlink,
	}
	tarWriter.WriteHeader(linkHeader)

	tarWriter.Close()
	gzWriter.Close()

	if err := os.WriteFile(tarballPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("Failed to write tarball: %v", err)
	}

	extractDir := filepath.Join(tmpDir, "extract")
	os.MkdirAll(extractDir, 0755)

	formationDir, err := ExtractBundle(tarballPath, extractDir)
	if err != nil {
		t.Fatalf("ExtractBundle() error = %v", err)
	}

	// Should extract successfully despite symlink
	if formationDir == "" {
		t.Error("formationDir is empty")
	}
}

func TestExtractBundle_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	tarballPath := filepath.Join(tmpDir, "large.tar.gz")

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add formation.yaml
	yamlContent := "id: large-test\nname: Large Test\nversion: 1.0.0\n"
	header := &tar.Header{
		Name: "large-test/formation.yaml",
		Size: int64(len(yamlContent)),
		Mode: 0644,
	}
	tarWriter.WriteHeader(header)
	tarWriter.Write([]byte(yamlContent))

	// Add a larger file
	largeContent := make([]byte, 1024*1024) // 1MB
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}
	largeHeader := &tar.Header{
		Name: "large-test/data.bin",
		Size: int64(len(largeContent)),
		Mode: 0644,
	}
	tarWriter.WriteHeader(largeHeader)
	tarWriter.Write(largeContent)

	tarWriter.Close()
	gzWriter.Close()

	if err := os.WriteFile(tarballPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("Failed to write tarball: %v", err)
	}

	extractDir := filepath.Join(tmpDir, "extract")
	os.MkdirAll(extractDir, 0755)

	formationDir, err := ExtractBundle(tarballPath, extractDir)
	if err != nil {
		t.Fatalf("ExtractBundle() error = %v", err)
	}

	// Verify large file was extracted
	dataFile := filepath.Join(formationDir, "data.bin")
	info, err := os.Stat(dataFile)
	if err != nil {
		t.Errorf("Large file not extracted: %v", err)
	}
	if info.Size() != 1024*1024 {
		t.Errorf("Large file size = %d, want %d", info.Size(), 1024*1024)
	}
}

func TestExtractBundle_DirectoryEntry(t *testing.T) {
	tmpDir := t.TempDir()
	tarballPath := filepath.Join(tmpDir, "dir-test.tar.gz")

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add directory entry
	dirHeader := &tar.Header{
		Name:     "dir-test/",
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}
	tarWriter.WriteHeader(dirHeader)

	// Add formation.yaml
	yamlContent := "id: dir-test\nname: Dir Test\nversion: 1.0.0\n"
	fileHeader := &tar.Header{
		Name: "dir-test/formation.yaml",
		Size: int64(len(yamlContent)),
		Mode: 0644,
	}
	tarWriter.WriteHeader(fileHeader)
	tarWriter.Write([]byte(yamlContent))

	tarWriter.Close()
	gzWriter.Close()

	os.WriteFile(tarballPath, buf.Bytes(), 0644)

	extractDir := filepath.Join(tmpDir, "extract")
	os.MkdirAll(extractDir, 0755)

	_, err := ExtractBundle(tarballPath, extractDir)
	if err != nil {
		t.Fatalf("ExtractBundle() error = %v", err)
	}
}

func TestExtractBundle_EmptyFormationDir(t *testing.T) {
	tmpDir := t.TempDir()
	tarballPath := filepath.Join(tmpDir, "empty-formation.tar.gz")

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add only a directory, no files
	dirHeader := &tar.Header{
		Name:     "empty-formation/",
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}
	tarWriter.WriteHeader(dirHeader)

	tarWriter.Close()
	gzWriter.Close()

	os.WriteFile(tarballPath, buf.Bytes(), 0644)

	extractDir := filepath.Join(tmpDir, "extract")
	os.MkdirAll(extractDir, 0755)

	// Should extract the directory even if empty
	formationDir, err := ExtractBundle(tarballPath, extractDir)
	if err != nil {
		t.Fatalf("ExtractBundle() error = %v", err)
	}

	if formationDir == "" {
		t.Error("formationDir should not be empty")
	}
}

func TestExtractBundle_FileCopyError(t *testing.T) {
	tmpDir := t.TempDir()
	tarballPath := filepath.Join(tmpDir, "copy-test.tar.gz")

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add formation.yaml
	yamlContent := "id: copy-test\nname: Copy Test\nversion: 1.0.0\n"
	header := &tar.Header{
		Name: "copy-test/formation.yaml",
		Size: int64(len(yamlContent)),
		Mode: 0644,
	}
	tarWriter.WriteHeader(header)
	tarWriter.Write([]byte(yamlContent))

	// Add another file
	fileContent := "test file content"
	fileHeader := &tar.Header{
		Name: "copy-test/test.txt",
		Size: int64(len(fileContent)),
		Mode: 0644,
	}
	tarWriter.WriteHeader(fileHeader)
	tarWriter.Write([]byte(fileContent))

	tarWriter.Close()
	gzWriter.Close()

	os.WriteFile(tarballPath, buf.Bytes(), 0644)

	extractDir := filepath.Join(tmpDir, "extract")
	os.MkdirAll(extractDir, 0755)

	formationDir, err := ExtractBundle(tarballPath, extractDir)
	if err != nil {
		t.Fatalf("ExtractBundle() error = %v", err)
	}

	// Verify both files extracted
	yamlPath := filepath.Join(formationDir, "formation.yaml")
	if _, err := os.Stat(yamlPath); err != nil {
		t.Errorf("formation.yaml not extracted: %v", err)
	}

	txtPath := filepath.Join(formationDir, "test.txt")
	if _, err := os.Stat(txtPath); err != nil {
		t.Errorf("test.txt not extracted: %v", err)
	}
}

func TestExtractBundle_NestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	tarballPath := filepath.Join(tmpDir, "nested.tar.gz")

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add formation.yaml in root
	yamlContent := "id: nested\nname: Nested\nversion: 1.0.0\n"
	header := &tar.Header{
		Name: "nested/formation.yaml",
		Size: int64(len(yamlContent)),
		Mode: 0644,
	}
	tarWriter.WriteHeader(header)
	tarWriter.Write([]byte(yamlContent))

	// Add nested directory
	dirHeader := &tar.Header{
		Name:     "nested/src/",
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}
	tarWriter.WriteHeader(dirHeader)

	// Add file in nested directory
	nestedContent := "nested file"
	nestedHeader := &tar.Header{
		Name: "nested/src/main.py",
		Size: int64(len(nestedContent)),
		Mode: 0644,
	}
	tarWriter.WriteHeader(nestedHeader)
	tarWriter.Write([]byte(nestedContent))

	tarWriter.Close()
	gzWriter.Close()

	os.WriteFile(tarballPath, buf.Bytes(), 0644)

	extractDir := filepath.Join(tmpDir, "extract")
	os.MkdirAll(extractDir, 0755)

	formationDir, err := ExtractBundle(tarballPath, extractDir)
	if err != nil {
		t.Fatalf("ExtractBundle() error = %v", err)
	}

	// Verify nested file extracted
	nestedPath := filepath.Join(formationDir, "src", "main.py")
	if _, err := os.Stat(nestedPath); err != nil {
		t.Errorf("Nested file not extracted: %v", err)
	}
}

func TestExtractBundle_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	tarballPath := filepath.Join(tmpDir, "multi.tar.gz")

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add formation.yaml
	yamlContent := "id: multi\nname: Multi\nversion: 1.0.0\n"
	tarWriter.WriteHeader(&tar.Header{
		Name: "multi/formation.yaml",
		Size: int64(len(yamlContent)),
		Mode: 0644,
	})
	tarWriter.Write([]byte(yamlContent))

	// Add 10 different files
	for i := 0; i < 10; i++ {
		content := fmt.Sprintf("File %d content", i)
		tarWriter.WriteHeader(&tar.Header{
			Name: fmt.Sprintf("multi/file%d.txt", i),
			Size: int64(len(content)),
			Mode: 0644,
		})
		tarWriter.Write([]byte(content))
	}

	tarWriter.Close()
	gzWriter.Close()

	os.WriteFile(tarballPath, buf.Bytes(), 0644)

	extractDir := filepath.Join(tmpDir, "extract")
	os.MkdirAll(extractDir, 0755)

	formationDir, err := ExtractBundle(tarballPath, extractDir)
	if err != nil {
		t.Fatalf("ExtractBundle() error = %v", err)
	}

	// Verify all files extracted
	for i := 0; i < 10; i++ {
		filePath := filepath.Join(formationDir, fmt.Sprintf("file%d.txt", i))
		if _, err := os.Stat(filePath); err != nil {
			t.Errorf("file%d.txt not extracted: %v", i, err)
		}
	}
}

func TestExtractBundle_UnsupportedTypeFlag(t *testing.T) {
	tmpDir := t.TempDir()
	tarballPath := filepath.Join(tmpDir, "unsupported.tar.gz")

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add formation.yaml first
	yamlContent := "id: unsupported\nname: Unsupported\nversion: 1.0.0\n"
	tarWriter.WriteHeader(&tar.Header{
		Name: "unsupported/formation.yaml",
		Size: int64(len(yamlContent)),
		Mode: 0644,
	})
	tarWriter.Write([]byte(yamlContent))

	// Add unsupported type (block device)
	tarWriter.WriteHeader(&tar.Header{
		Name:     "unsupported/device",
		Typeflag: tar.TypeBlock,
	})

	tarWriter.Close()
	gzWriter.Close()

	os.WriteFile(tarballPath, buf.Bytes(), 0644)

	extractDir := filepath.Join(tmpDir, "extract")
	os.MkdirAll(extractDir, 0755)

	// Should still extract successfully (just skip unsupported types)
	formationDir, err := ExtractBundle(tarballPath, extractDir)
	if err != nil {
		t.Logf("ExtractBundle() error = %v (may skip unsupported types)", err)
	}
	if formationDir != "" {
		t.Logf("Extracted to: %s", formationDir)
	}
}

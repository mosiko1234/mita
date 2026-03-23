package crypto

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCalculateFileSHA256(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world\n"), 0644); err != nil {
		t.Fatal(err)
	}

	hash, err := CalculateFileSHA256(testFile)
	if err != nil {
		t.Fatal(err)
	}

	// sha256("hello world\n")
	expected := "a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447"
	if hash != expected {
		t.Errorf("got %s, want %s", hash, expected)
	}
}

func TestGenerateAndVerifyChecksum(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"a.txt": "file a content",
		"b.txt": "file b content",
	}
	var names []string
	for name, content := range files {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}

	// Generate checksum file
	checksumPath := filepath.Join(tmpDir, "checksum.sha256")
	if err := GenerateChecksumFile(tmpDir, names, checksumPath); err != nil {
		t.Fatal(err)
	}

	// Verify passes
	if err := VerifyChecksumFile(checksumPath, tmpDir); err != nil {
		t.Fatalf("verification should pass: %v", err)
	}

	// Tamper with a file and verify fails
	if err := os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyChecksumFile(checksumPath, tmpDir); err == nil {
		t.Fatal("verification should fail after tampering")
	}
}

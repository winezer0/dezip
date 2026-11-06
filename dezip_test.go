package dezip

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type testArchiveEntry struct {
	name    string
	content string
}

func TestParseExtractAs(t *testing.T) {
	tests := []struct {
		value string
		want  Mode
	}{
		{"", ModeSkip},
		{"skip", ModeSkip},
		{"auto", ModeAuto},
		{"zip", ModeZip},
		{"tar", ModeTar},
		{"tar.gz", ModeTar},
		{"gzip", ModeGzip},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			got, err := ParseExtractAs(test.value)
			if err != nil {
				t.Fatalf("ParseExtractAs(%q): %v", test.value, err)
			}
			if got != test.want {
				t.Fatalf("ParseExtractAs(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}

	if _, err := ParseExtractAs("exe"); !errors.Is(err, ErrInvalidMode) {
		t.Fatalf("ParseExtractAs(\"exe\") error = %v, want ErrInvalidMode", err)
	}
	if _, err := normalizedMode(Mode("exe")); !errors.Is(err, ErrInvalidMode) {
		t.Fatalf("normalizedMode(\"exe\") error = %v, want ErrInvalidMode", err)
	}
}

func TestProcessSkipKeepsDownloadedFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "plugin.exe")
	destination := filepath.Join(root, "output")
	writeTestFile(t, source, "binary")

	result, err := Process(context.Background(), source, destination, Mode(""), Options{})
	if err != nil {
		t.Fatalf("Process(skip): %v", err)
	}
	if !result.Skipped || result.Mode != ModeSkip {
		t.Fatalf("Process(skip) result = %+v, want skipped mode", result)
	}
	if result.SourcePath != source {
		t.Fatalf("SourcePath = %q, want %q", result.SourcePath, source)
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination stat error = %v, want not-exist", err)
	}
}

func TestProcessZipExtractsNestedFilesAndResolvesExpectedFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "plugin.zip")
	destination := filepath.Join(root, "plugin")
	writeTestZip(t, source, []testArchiveEntry{
		{name: "bin/plugin.exe", content: "binary"},
		{name: "data/config.json", content: "{}"},
	})

	result, err := Process(context.Background(), source, destination, ModeZip, Options{ExpectedFile: "bin/plugin.exe"})
	if err != nil {
		t.Fatalf("Process(zip): %v", err)
	}
	if result.Format != "zip" || result.Files != 2 {
		t.Fatalf("result format/files = %q/%d, want zip/2", result.Format, result.Files)
	}
	if result.OutputDir != destination {
		t.Fatalf("OutputDir = %q, want %q", result.OutputDir, destination)
	}
	wantMainFile := filepath.Join(destination, "bin", "plugin.exe")
	if result.OutputFile != wantMainFile {
		t.Fatalf("OutputFile = %q, want %q", result.OutputFile, wantMainFile)
	}
	assertFileContent(t, wantMainFile, "binary")
	assertFileContent(t, filepath.Join(destination, "data", "config.json"), "{}")
}

func TestProcessTarGzipExtractsMultipleFiles(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "plugin.tar.gz")
	destination := filepath.Join(root, "plugin")
	writeTestTarGzip(t, source, []testArchiveEntry{
		{name: "root/plugin.bin", content: "binary"},
		{name: "root/assets/icon.dat", content: "icon"},
	})

	result, err := Process(context.Background(), source, destination, ModeTar, Options{StripComponents: 1})
	if err != nil {
		t.Fatalf("Process(tar.gz): %v", err)
	}
	if result.Files != 2 {
		t.Fatalf("Files = %d, want 2", result.Files)
	}
	assertFileContent(t, filepath.Join(destination, "plugin.bin"), "binary")
	assertFileContent(t, filepath.Join(destination, "assets", "icon.dat"), "icon")
}

func TestProcessGzipWritesSingleFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "plugin.bin.gz")
	destination := filepath.Join(root, "plugin")
	writeTestGzip(t, source, "binary")

	result, err := Process(context.Background(), source, destination, ModeGzip, Options{})
	if err != nil {
		t.Fatalf("Process(gzip): %v", err)
	}
	wantFile := filepath.Join(destination, "plugin.bin")
	if result.OutputFile != wantFile || result.Files != 1 {
		t.Fatalf("result output/files = %q/%d, want %q/1", result.OutputFile, result.Files, wantFile)
	}
	assertFileContent(t, wantFile, "binary")
}

func TestProcessRejectsTraversalAndCleansStagingDirectory(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "unsafe.zip")
	destination := filepath.Join(root, "output")
	writeTestZip(t, source, []testArchiveEntry{{name: "../escape.txt", content: "bad"}})

	_, err := Process(context.Background(), source, destination, ModeZip, Options{})
	if !errors.Is(err, ErrUnsafeEntry) {
		t.Fatalf("Process(unsafe zip) error = %v, want ErrUnsafeEntry", err)
	}
	assertUnpublishedAndNoStaging(t, destination)
	if _, err := os.Stat(filepath.Join(root, "escape.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("escaped file stat error = %v, want not-exist", err)
	}
}

func TestProcessMissingExpectedFileCleansStagingDirectory(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "plugin.zip")
	destination := filepath.Join(root, "output")
	writeTestZip(t, source, []testArchiveEntry{{name: "present.txt", content: "content"}})

	_, err := Process(context.Background(), source, destination, ModeZip, Options{ExpectedFile: "missing.exe"})
	if err == nil {
		t.Fatal("Process() succeeded without the expected output file")
	}
	assertUnpublishedAndNoStaging(t, destination)
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeTestZip(t *testing.T, path string, entries []testArchiveEntry) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	for _, entry := range entries {
		member, err := writer.Create(entry.name)
		if err != nil {
			t.Fatalf("create zip member %q: %v", entry.name, err)
		}
		if _, err := member.Write([]byte(entry.content)); err != nil {
			t.Fatalf("write zip member %q: %v", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
}

func writeTestTarGzip(t *testing.T, path string, entries []testArchiveEntry) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tar.gz: %v", err)
	}
	defer file.Close()

	compressed := gzip.NewWriter(file)
	writer := tar.NewWriter(compressed)
	for _, entry := range entries {
		if err := writer.WriteHeader(&tar.Header{Name: entry.name, Mode: 0o644, Size: int64(len(entry.content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatalf("write tar header %q: %v", entry.name, err)
		}
		if _, err := writer.Write([]byte(entry.content)); err != nil {
			t.Fatalf("write tar member %q: %v", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
}

func writeTestGzip(t *testing.T, path, content string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create gzip: %v", err)
	}
	defer file.Close()

	writer := gzip.NewWriter(file)
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatalf("write gzip: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(content) != want {
		t.Fatalf("content of %s = %q, want %q", path, content, want)
	}
}

func assertUnpublishedAndNoStaging(t *testing.T, destination string) {
	t.Helper()
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination stat error = %v, want not-exist", err)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(destination), ".dezip-*"))
	if err != nil {
		t.Fatalf("find staging directories: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("staging directories remain after failure: %v", matches)
	}
}

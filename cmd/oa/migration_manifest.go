package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/HMB-research/open-accounting/internal/cutover"
)

func buildMigrationBundleFilesWithManifest(manifestPath string, inputs []migrationFileInput) ([]cutover.BundleFile, error) {
	manifestFiles, err := readMigrationBundleFilesFromManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	flagFiles, err := buildMigrationBundleFiles(inputs)
	if err != nil {
		return nil, err
	}
	files := append(manifestFiles, flagFiles...)
	if err := rejectDuplicateMigrationBundleFiles(files); err != nil {
		return nil, err
	}
	return files, nil
}

func readMigrationBundleFilesFromManifest(path string) ([]cutover.BundleFile, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read migration manifest: %w", err)
	}
	var manifest cutover.SmartAccountsSnapshotReport
	if err := json.Unmarshal(content, &manifest); err != nil {
		return nil, fmt.Errorf("parse migration manifest: %w", err)
	}
	if len(manifest.PreparedFiles) == 0 {
		return nil, fmt.Errorf("migration manifest has no prepared files")
	}

	manifestDir := filepath.Dir(path)
	files := make([]cutover.BundleFile, 0, len(manifest.PreparedFiles))
	loadedOutputs := map[string]struct{}{}
	for _, prepared := range manifest.PreparedFiles {
		outputPath, err := resolveMigrationManifestOutputPath(manifestDir, prepared.OutputPath)
		if err != nil {
			return nil, err
		}
		outputKey := string(prepared.Kind) + "\x00" + outputPath
		if _, ok := loadedOutputs[outputKey]; ok {
			continue
		}
		loadedOutputs[outputKey] = struct{}{}
		payload, err := os.ReadFile(outputPath)
		if err != nil {
			return nil, fmt.Errorf("read manifest bundle file %s: %w", prepared.OutputPath, err)
		}
		if expected := strings.TrimSpace(prepared.OutputSHA256); expected != "" {
			actual := sha256HexForMigrationManifest(payload)
			if !strings.EqualFold(actual, expected) {
				return nil, fmt.Errorf("manifest bundle file %s sha256 mismatch", prepared.OutputPath)
			}
		}
		file := cutover.BundleFile{
			Kind:     prepared.Kind,
			FileName: filepath.Base(outputPath),
		}
		if prepared.Kind == cutover.KindEInvoices || strings.EqualFold(filepath.Ext(outputPath), ".xml") {
			file.XMLContent = string(payload)
		} else {
			file.CSVContent = string(payload)
		}
		files = append(files, file)
	}
	return files, nil
}

func resolveMigrationManifestOutputPath(manifestDir, outputPath string) (string, error) {
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		return "", fmt.Errorf("migration manifest prepared file is missing output_path")
	}
	if filepath.IsAbs(outputPath) {
		return filepath.Clean(outputPath), nil
	}
	target := filepath.Clean(filepath.Join(manifestDir, outputPath))
	rel, _ := filepath.Rel(manifestDir, target)
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("migration manifest bundle file %s escapes manifest directory", outputPath)
	}
	return target, nil
}

func rejectDuplicateMigrationBundleFiles(files []cutover.BundleFile) error {
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		key := migrationStepFileKey(file.Kind, file.FileName)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate migration bundle file %s (%s)", file.FileName, file.Kind)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func sha256HexForMigrationManifest(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

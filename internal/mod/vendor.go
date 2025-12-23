package mod

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/sourcegraph/conc/pool"
)

const (
	goModFile  = "go.mod"
	vendorFile = "govendor.toml"
)

type vendorOptions struct {
	detectDrift bool
	paths       []string
	recursive   bool
	maxDepth    int
}

type VendorOption func(*vendorOptions)

func WithDriftDetection() VendorOption {
	return func(opts *vendorOptions) {
		opts.detectDrift = true
	}
}

func WithPaths(paths ...string) VendorOption {
	return func(opts *vendorOptions) {
		for _, path := range paths {
			if base := filepath.Base(path); base == goModFile || base == vendorFile {
				path = filepath.Dir(path)
			}
			opts.paths = append(opts.paths, path)
		}
	}
}

func WithRecursive(maxDepth int) VendorOption {
	return func(opts *vendorOptions) {
		opts.recursive = true
		opts.maxDepth = maxDepth
	}
}

type Vendor struct {
	opts vendorOptions
}

func NewVendor(opts ...VendorOption) *Vendor {
	v := &Vendor{}
	for _, opt := range opts {
		opt(&v.opts)
	}
	return v
}

var errVendorFailed = fmt.Errorf("vendor failed")

func (v *Vendor) VendorFiles() error {
	modFiles, missingResults := v.findModFiles()

	p := pool.NewWithResults[vendorResult]()
	for _, modFile := range modFiles {
		p.Go(func() vendorResult {
			return v.processModFile(modFile)
		})
	}

	results := append(missingResults, p.Wait()...)

	sort.Slice(results, func(i, j int) bool {
		return results[i].path < results[j].path
	})

	fmt.Println(renderResultsTable(results))

	for _, r := range results {
		if r.status.IsFailure() {
			return errVendorFailed
		}
	}

	return nil
}

func (v *Vendor) findModFiles() (modFiles []string, missing []vendorResult) {
	paths := v.opts.paths
	if len(paths) == 0 {
		paths = []string{"."}
	}

	if v.opts.recursive {
		p := pool.NewWithResults[[]string]().WithErrors()
		for _, path := range paths {
			p.Go(func() ([]string, error) {
				scanner := NewFileScanner(WithMaxDepth(v.opts.maxDepth))
				return scanner.ScanFrom(path)
			})
		}

		results, err := p.Wait()
		if err != nil {
			return nil, nil
		}

		for _, found := range results {
			modFiles = append(modFiles, found...)
		}

		if len(modFiles) == 0 {
			for _, path := range paths {
				missing = append(missing, resultNotFound(filepath.Join(path, goModFile)))
			}
		}

		return modFiles, missing
	}

	for _, path := range paths {
		modPath := filepath.Join(path, goModFile)
		if _, err := os.Stat(modPath); err != nil {
			missing = append(missing, resultNotFound(modPath))
		} else {
			modFiles = append(modFiles, modPath)
		}
	}

	return modFiles, missing
}

func (v *Vendor) processModFile(path string) vendorResult {
	goMod, err := ParseGoModFile(path)
	if err != nil {
		return resultError(path, err)
	}

	if !goMod.HasDependencies() {
		return resultSkipped(path)
	}

	vendorPath := filepath.Join(goMod.Dir(), vendorFile)
	existingData, err := os.ReadFile(vendorPath)

	if os.IsNotExist(err) {
		if v.opts.detectDrift {
			return resultMissing(path)
		}
	} else if err != nil {
		return resultError(path, err)
	} else {
		existingHash, err := extractHash(existingData)
		if err != nil {
			return resultError(path, err)
		}

		if existingHash == goMod.Hash() {
			return resultOK(path)
		}

		if v.opts.detectDrift {
			return resultDrift(path)
		}
	}

	depCount, err := v.generateManifest(goMod)
	if err != nil {
		return resultError(path, err)
	}

	return resultGenerated(path, depCount)
}

func (v *Vendor) generateManifest(goMod *GoModFile) (int, error) {
	manifest, err := newManifest(goMod)
	if err != nil {
		return 0, err
	}

	var buf bytes.Buffer
	if _, err := manifest.WriteTo(&buf); err != nil {
		return 0, err
	}

	outputPath := filepath.Join(goMod.Dir(), vendorFile)
	if err := os.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
		return 0, err
	}

	return len(manifest.Mod), nil
}

package mod

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/sourcegraph/conc/pool"
)

var (
	ErrNoModFile    = errors.New("no go.mod file found")
	ErrVendorFailed = errors.New("one or more modules failed")
)

const (
	goModFile  = "go.mod"
	vendorFile = "govendor.toml"
)

type vendorStatus string

const (
	statusOK        vendorStatus = "ok"
	statusGenerated vendorStatus = "generated"
	statusDrift     vendorStatus = "drift"
	statusMissing   vendorStatus = "missing"
	statusSkipped   vendorStatus = "skipped"
	statusError     vendorStatus = "error"
)

func (s vendorStatus) IsSuccess() bool {
	return s == statusOK || s == statusGenerated || s == statusSkipped
}

type vendorResult struct {
	path    string
	status  vendorStatus
	message string
}

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

func (v *Vendor) VendorFiles() error {
	modFiles, err := v.findModFiles()
	if err != nil {
		return err
	}

	p := pool.NewWithResults[vendorResult]()
	for _, modFile := range modFiles {
		p.Go(func() vendorResult {
			return v.processDir(modFile)
		})
	}

	results := p.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].path < results[j].path
	})

	var failed bool
	for _, r := range results {
		if r.message != "" {
			fmt.Printf("%s: %s (%s)\n", r.path, r.status, r.message)
		} else {
			fmt.Printf("%s: %s\n", r.path, r.status)
		}

		if r.status == statusSkipped {
			continue
		}

		if !r.status.IsSuccess() {
			failed = true
		}
	}

	if failed {
		return ErrVendorFailed
	}

	return nil
}

func (v *Vendor) findModFiles() ([]string, error) {
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
			return nil, err
		}

		var modFiles []string
		for _, found := range results {
			modFiles = append(modFiles, found...)
		}

		if len(modFiles) == 0 {
			return nil, ErrNoModFile
		}

		return modFiles, nil
	}

	var modFiles []string
	for _, path := range paths {
		modPath := filepath.Join(path, goModFile)
		if _, err := os.Stat(modPath); err != nil {
			if os.IsNotExist(err) {
				return nil, ErrNoModFile
			}
			return nil, err
		}
		modFiles = append(modFiles, modPath)
	}

	return modFiles, nil
}

func (v *Vendor) processDir(path string) vendorResult {
	result := vendorResult{path: path}

	goMod, err := ParseGoModFile(path)
	if err != nil {
		result.status = statusError
		result.message = err.Error()
		return result
	}

	if !goMod.HasDependencies() {
		result.status = statusSkipped
		result.message = "no dependencies"
		return result
	}

	vendorPath := filepath.Join(goMod.Dir(), vendorFile)
	existingData, err := os.ReadFile(vendorPath)
	if err == nil {
		existingHash, err := extractHash(existingData)
		if err != nil {
			result.status = statusError
			result.message = err.Error()
			return result
		}

		if existingHash == goMod.Hash() {
			result.status = statusOK
			return result
		}

		if v.opts.detectDrift {
			result.status = statusDrift
			return result
		}
	} else if os.IsNotExist(err) {
		if v.opts.detectDrift {
			result.status = statusMissing
			return result
		}
	} else {
		result.status = statusError
		result.message = err.Error()
		return result
	}

	if err := v.generateManifest(goMod); err != nil {
		result.status = statusError
		result.message = err.Error()
		return result
	}

	result.status = statusGenerated
	return result
}

func (v *Vendor) generateManifest(goMod *GoModFile) error {
	manifest, err := newManifest(goMod)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if _, err := manifest.WriteTo(&buf); err != nil {
		return err
	}

	outputPath := filepath.Join(goMod.Dir(), vendorFile)
	return os.WriteFile(outputPath, buf.Bytes(), 0o644)
}

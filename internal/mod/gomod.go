package mod

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"

	"github.com/nix-community/go-nix/pkg/nar"
	"github.com/sourcegraph/conc/pool"
	"golang.org/x/mod/modfile"
)

//nolint:tagliatelle
type goModuleDownload struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
	Dir     string `json:"Dir"`
	GoMod   string `json:"GoMod"`
}

type GoModFile struct {
	dir     string
	content []byte
	modfile *modfile.File
	hash    string
}

func ParseGoModFile(path string) (*GoModFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.mod: %w", err)
	}

	mf, err := modfile.Parse(path, content, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.mod: %w", err)
	}

	h := sha256.Sum256(content)
	hash := "sha256-" + base64.StdEncoding.EncodeToString(h[:])

	return &GoModFile{
		dir:     filepath.Dir(path),
		content: content,
		modfile: mf,
		hash:    hash,
	}, nil
}

func (f *GoModFile) Dir() string {
	return f.dir
}

func (f *GoModFile) Hash() string {
	return f.hash
}

func (f *GoModFile) ModulePath() string {
	return f.modfile.Module.Mod.Path
}

func (f *GoModFile) HasDependencies() bool {
	return len(f.modfile.Require) > 0
}

type Replacement struct {
	OldPath   string
	NewPath   string
	IsLocal   bool
	LocalPath string
}

func (f *GoModFile) Replacements() map[string]Replacement {
	replacements := make(map[string]Replacement)
	for _, repl := range f.modfile.Replace {
		isLocal := strings.HasPrefix(repl.New.Path, ".") || strings.HasPrefix(repl.New.Path, "/")
		replacements[repl.New.Path] = Replacement{
			OldPath:   repl.Old.Path,
			NewPath:   repl.New.Path,
			IsLocal:   isLocal,
			LocalPath: repl.New.Path,
		}
	}
	return replacements
}

func (f *GoModFile) Dependencies(platforms []string) ([]GoModule, error) {
	if platforms == nil {
		platforms = DefaultPlatforms
	}
	pkgsByMod, err := f.packagesByModule(platforms)
	if err != nil {
		return nil, err
	}

	downloads, err := f.downloadModules()
	if err != nil {
		return nil, err
	}

	modules, err := f.resolveModules(downloads, pkgsByMod)
	if err != nil {
		return nil, err
	}

	localModules, err := f.resolveLocalModules(pkgsByMod)
	if err != nil {
		return nil, err
	}

	modules = append(modules, localModules...)
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Path < modules[j].Path
	})

	return modules, nil
}

// DefaultPlatforms is the default set of platforms used for cross-platform
// dependency resolution. Callers can pass a subset to Dependencies to
// restrict resolution to specific platforms.
var DefaultPlatforms = []string{
	"linux/amd64",
	"linux/arm64",
	"darwin/amd64",
	"darwin/arm64",
	"windows/amd64",
	"windows/arm64",
}

func (f *GoModFile) packagesByModule(platforms []string) (map[string][]string, error) {
	current, err := f.packagesByModuleForPlatform(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, err
	}

	p := pool.NewWithResults[map[string][]string]().WithErrors()
	for _, plat := range platforms {
		parts := strings.Split(plat, "/")
		if len(parts) != 2 {
			continue
		}
		goos, goarch := parts[0], parts[1]
		if goos == runtime.GOOS && goarch == runtime.GOARCH {
			continue
		}
		p.Go(func() (map[string][]string, error) {
			return f.packagesByModuleForPlatform(goos, goarch)
		})
	}

	results, err := p.Wait()
	if err != nil {
		return nil, err
	}

	merged := current
	for _, result := range results {
		for mod, pkgs := range result {
			merged[mod] = append(merged[mod], pkgs...)
		}
	}

	for modPath := range merged {
		sort.Strings(merged[modPath])
		merged[modPath] = slices.Compact(merged[modPath])
	}

	return merged, nil
}

func (f *GoModFile) packagesByModuleForPlatform(goos, goarch string) (map[string][]string, error) {
	listFmt := fmt.Sprintf(`'{{if not .Standard}}{{if .Module}}{{if ne .Module.Path "%s"}}{{.Module.Path}}{{"\t"}}{{.ImportPath}}{{end}}{{end}}{{end}}'`, f.ModulePath())

	cmd := []string{
		"go", "list", "-deps", "-test", "-f", listFmt, "./...",
	}

	// GOWORK=off ensures this module is processed independently, which is
	// essential for workspaces where each module's dependencies must be
	// resolved in isolation before being merged at the workspace level.
	env := []string{
		"GOWORK=off",
		"GOOS=" + goos,
		"GOARCH=" + goarch,
	}

	out, err := execWithEnv(cmd, f.dir, env)
	if err != nil {
		return nil, err
	}

	pkgsByMod := parsePackagesByModule(out)

	// Include tool dependencies (Go 1.24+) so their packages appear in the
	// module-to-package mapping and are listed in modules.txt. A separate
	// invocation without -test avoids pulling in each tool's test-only
	// dependencies.
	if len(f.modfile.Tool) > 0 {
		toolCmd := []string{
			"go", "list", "-deps", "-f", listFmt, "tool",
		}

		toolOut, err := execWithEnv(toolCmd, f.dir, env)
		if err != nil {
			return nil, err
		}

		for mod, pkgs := range parsePackagesByModule(toolOut) {
			pkgsByMod[mod] = append(pkgsByMod[mod], pkgs...)
		}
	}

	return pkgsByMod, nil
}

func parsePackagesByModule(out string) map[string][]string {
	pkgsByMod := make(map[string][]string)
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		modPath, pkgPath, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		pkgsByMod[modPath] = append(pkgsByMod[modPath], pkgPath)
	}
	return pkgsByMod
}

func (f *GoModFile) downloadModules() ([]goModuleDownload, error) {
	cmd := []string{
		"go",
		"mod",
		"download",
		"-json",
	}

	// GOWORK=off ensures this module is processed independently (see packagesByModuleForPlatform)
	out, err := execWithEnv(cmd, f.dir, []string{"GOWORK=off"})
	if err != nil {
		return nil, err
	}

	return parseDownloadOutput(out)
}

func parseDownloadOutput(out string) ([]goModuleDownload, error) {
	var downloads []goModuleDownload
	dec := json.NewDecoder(strings.NewReader(out))
	for {
		var meta goModuleDownload
		if err := dec.Decode(&meta); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		downloads = append(downloads, meta)
	}
	return downloads, nil
}

func (f *GoModFile) resolveModules(downloads []goModuleDownload, pkgsByMod map[string][]string) ([]GoModule, error) {
	replacements := f.Replacements()

	p := pool.NewWithResults[GoModule]().WithErrors().WithMaxGoroutines(8)

	for _, meta := range downloads {
		p.Go(func() (GoModule, error) {
			h := sha256.New()
			if err := nar.DumpPath(h, meta.Dir); err != nil {
				return GoModule{}, err
			}

			digest := h.Sum(nil)
			hash := "sha256-" + base64.StdEncoding.EncodeToString(digest)

			var goVersion string
			if meta.GoMod != "" {
				if modData, err := os.ReadFile(meta.GoMod); err == nil {
					if modFile, err := modfile.Parse(meta.GoMod, modData, nil); err == nil && modFile.Go != nil {
						goVersion = modFile.Go.Version
					}
				}
			}

			path := meta.Path
			var replacedPath string
			if repl, ok := replacements[path]; ok {
				path = repl.OldPath
				replacedPath = meta.Path
			}

			return GoModule{
				Path:         path,
				Version:      meta.Version,
				Packages:     pkgsByMod[meta.Path],
				Hash:         hash,
				GoVersion:    goVersion,
				ReplacedPath: replacedPath,
			}, nil
		})
	}

	goModules, err := p.Wait()
	if err != nil {
		return nil, err
	}

	return goModules, nil
}

func (f *GoModFile) resolveLocalModules(pkgsByMod map[string][]string) ([]GoModule, error) {
	replacements := f.Replacements()

	var localRepls []Replacement
	for _, repl := range replacements {
		if repl.IsLocal {
			localRepls = append(localRepls, repl)
		}
	}

	if len(localRepls) == 0 {
		return nil, nil
	}

	requires := make(map[string]string)
	for _, req := range f.modfile.Require {
		requires[req.Mod.Path] = req.Mod.Version
	}

	p := pool.NewWithResults[GoModule]().WithErrors().WithMaxGoroutines(8)

	for _, repl := range localRepls {
		p.Go(func() (GoModule, error) {
			localDir := filepath.Join(f.dir, repl.LocalPath)
			localDir, err := filepath.Abs(localDir)
			if err != nil {
				return GoModule{}, fmt.Errorf("failed to resolve local module path %s: %w", repl.LocalPath, err)
			}

			h := sha256.New()
			if err := nar.DumpPathFilter(h, localDir, func(path string, _ nar.NodeType) bool {
				return strings.ToLower(filepath.Base(path)) != ".ds_store"
			}); err != nil {
				return GoModule{}, fmt.Errorf("failed to hash local module %s: %w", repl.LocalPath, err)
			}

			digest := h.Sum(nil)
			hash := "sha256-" + base64.StdEncoding.EncodeToString(digest)

			var goVersion string
			localGoMod := filepath.Join(localDir, "go.mod")
			if modData, err := os.ReadFile(localGoMod); err == nil {
				if modFile, err := modfile.Parse(localGoMod, modData, nil); err == nil && modFile.Go != nil {
					goVersion = modFile.Go.Version
				}
			}

			version := requires[repl.OldPath]
			if version == "" {
				version = "v0.0.0"
			}

			return GoModule{
				Path:         repl.OldPath,
				Version:      version,
				Packages:     pkgsByMod[repl.OldPath],
				Hash:         hash,
				GoVersion:    goVersion,
				ReplacedPath: repl.OldPath,
				Local:        repl.LocalPath,
			}, nil
		})
	}

	return p.Wait()
}

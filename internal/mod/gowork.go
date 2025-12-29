package mod

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"

	"github.com/sourcegraph/conc/pool"
	"golang.org/x/mod/modfile"
)

type GoWorkFile struct {
	dir      string
	content  []byte
	workfile *modfile.WorkFile
	hash     string
	modules  []string
}

func ParseGoWorkFile(path string) (*GoWorkFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.work: %w", err)
	}

	wf, err := modfile.ParseWork(path, content, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.work: %w", err)
	}

	h := sha256.Sum256(content)
	hash := "sha256-" + base64.StdEncoding.EncodeToString(h[:])

	var modules []string
	for _, use := range wf.Use {
		modules = append(modules, use.Path)
	}

	return &GoWorkFile{
		dir:      filepath.Dir(path),
		content:  content,
		workfile: wf,
		hash:     hash,
		modules:  modules,
	}, nil
}

func (w *GoWorkFile) Dir() string {
	return w.dir
}

func (w *GoWorkFile) Hash() string {
	return w.hash
}

func (w *GoWorkFile) Modules() []string {
	return w.modules
}

func (w *GoWorkFile) ModulePaths() []string {
	var paths []string
	for _, mod := range w.modules {
		paths = append(paths, filepath.Join(w.dir, mod))
	}
	return paths
}

func (w *GoWorkFile) Dependencies(extraPlatforms []string) ([]GoModule, error) {
	pkgsByMod, err := w.packagesByModule(extraPlatforms)
	if err != nil {
		return nil, err
	}

	downloads, err := w.downloadModules()
	if err != nil {
		return nil, err
	}

	modules, err := w.resolveModules(downloads, pkgsByMod)
	if err != nil {
		return nil, err
	}

	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Path < modules[j].Path
	})

	return modules, nil
}

func (w *GoWorkFile) packagesByModule(extraPlatforms []string) (map[string][]string, error) {
	current, err := w.packagesByModuleForPlatform(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, err
	}

	platforms := make([]platform, len(defaultPlatforms))
	copy(platforms, defaultPlatforms)
	for _, ep := range extraPlatforms {
		parts := strings.Split(ep, "/")
		if len(parts) == 2 {
			platforms = append(platforms, platform{parts[0], parts[1]})
		}
	}

	p := pool.NewWithResults[map[string][]string]().WithErrors()
	for _, plat := range platforms {
		if plat.goos == runtime.GOOS && plat.goarch == runtime.GOARCH {
			continue
		}
		p.Go(func() (map[string][]string, error) {
			return w.packagesByModuleForPlatform(plat.goos, plat.goarch)
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

func (w *GoWorkFile) packagesByModuleForPlatform(goos, goarch string) (map[string][]string, error) {
	workspaceModules := w.workspaceModulePaths()
	excludeExpr := w.buildExcludeExpression(workspaceModules)

	cmd := []string{
		"go",
		"list",
		"-deps",
		"-f",
		fmt.Sprintf(`'{{if not .Standard}}{{if .Module}}%s{{.Module.Path}}{{"\t"}}{{.ImportPath}}{{end}}{{end}}{{end}}'`, excludeExpr),
	}

	for _, mod := range w.modules {
		cmd = append(cmd, "./"+mod+"/...")
	}

	env := []string{
		"GOOS=" + goos,
		"GOARCH=" + goarch,
	}

	out, err := execWithEnv(cmd, w.dir, env)
	if err != nil {
		return nil, err
	}

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

	return pkgsByMod, nil
}

func (w *GoWorkFile) workspaceModulePaths() []string {
	var paths []string
	for _, mod := range w.modules {
		modPath := filepath.Join(w.dir, mod, goModFile)
		if content, err := os.ReadFile(modPath); err == nil {
			if mf, err := modfile.Parse(modPath, content, nil); err == nil {
				paths = append(paths, mf.Module.Mod.Path)
			}
		}
	}
	return paths
}

func (w *GoWorkFile) WorkspaceDependencies() map[string]WorkspaceMember {
	dirToModPath := make(map[string]string)
	modPathToDir := make(map[string]string)
	modPathToGoVersion := make(map[string]string)

	for _, mod := range w.modules {
		modFilePath := filepath.Join(w.dir, mod, goModFile)
		if content, err := os.ReadFile(modFilePath); err == nil {
			if mf, err := modfile.Parse(modFilePath, content, nil); err == nil {
				dirToModPath[mod] = mf.Module.Mod.Path
				modPathToDir[mf.Module.Mod.Path] = mod
				if mf.Go != nil {
					modPathToGoVersion[mf.Module.Mod.Path] = mf.Go.Version
				}
			}
		}
	}

	workspaceModSet := make(map[string]bool)
	for _, modPath := range dirToModPath {
		workspaceModSet[modPath] = true
	}

	importedByOthers := make(map[string]bool)
	for _, mod := range w.modules {
		thisModPath := dirToModPath[mod]

		cmd := []string{
			"go",
			"list",
			"-deps",
			"-f",
			"'{{if .Module}}{{.Module.Path}}{{end}}'",
			"./" + mod + "/...",
		}

		out, err := exec(cmd, w.dir)
		if err != nil {
			continue
		}

		for line := range strings.SplitSeq(out, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && workspaceModSet[line] && line != thisModPath {
				importedByOthers[line] = true
			}
		}
	}

	members := make(map[string]WorkspaceMember)
	for modPath := range importedByOthers {
		members[modPath] = WorkspaceMember{
			Path:      modPathToDir[modPath],
			GoVersion: modPathToGoVersion[modPath],
		}
	}
	return members
}

func (w *GoWorkFile) buildExcludeExpression(modulePaths []string) string {
	if len(modulePaths) == 0 {
		return ""
	}

	var conditions []string
	for _, path := range modulePaths {
		conditions = append(conditions, fmt.Sprintf(`ne .Module.Path "%s"`, path))
	}

	return fmt.Sprintf("{{if and (%s)}}", strings.Join(conditions, ") ("))
}

func (w *GoWorkFile) downloadModules() ([]goModuleDownload, error) {
	cmd := []string{
		"go",
		"mod",
		"download",
		"-json",
	}

	out, err := exec(cmd, w.dir)
	if err != nil {
		return nil, err
	}

	return parseDownloadOutput(out)
}

func (w *GoWorkFile) resolveModules(downloads []goModuleDownload, pkgsByMod map[string][]string) ([]GoModule, error) {
	p := pool.NewWithResults[GoModule]().WithErrors().WithMaxGoroutines(8)

	for _, meta := range downloads {
		p.Go(func() (GoModule, error) {
			return resolveModuleFromDownload(meta, pkgsByMod)
		})
	}

	return p.Wait()
}

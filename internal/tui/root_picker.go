package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ovw/internal/config"
)

// rootPicker selects config roots, shown to users as project folders.
type rootPicker struct {
	options  []string
	checked  map[string]bool
	expanded map[string]bool
	children map[string][]string
	counts   map[string]int
	selected int
}

func newRootPicker(candidates, roots []string) rootPicker {
	picker := rootPicker{
		options:  append([]string{}, candidates...),
		checked:  checkedRootPickerOptions(candidates, roots),
		expanded: map[string]bool{},
		children: map[string][]string{},
		counts:   map[string]int{},
	}
	picker.revealChecked()
	return picker
}

func checkedRootPickerOptions(candidates, roots []string) map[string]bool {
	if len(roots) == 0 {
		return checkedOnboardingOptions(candidates)
	}
	checked := make(map[string]bool, len(roots))
	for _, root := range roots {
		checked[root] = true
	}
	return checked
}

func rootPickerOptionsWithRoots(candidates, roots []string) []string {
	seen := map[string]bool{}
	options := make([]string, 0, len(candidates)+len(roots))
	for _, path := range candidates {
		if path == "" || seen[path] {
			continue
		}
		options = append(options, path)
		seen[path] = true
	}
	for _, path := range roots {
		if path == "" || seen[path] || rootPickerHasOptionAncestor(options, path) {
			continue
		}
		options = append(options, path)
		seen[path] = true
	}
	return options
}

func rootPickerHasOptionAncestor(options []string, path string) bool {
	for _, option := range options {
		if normalizeRootPickerPath(option) == normalizeRootPickerPath(path) || rootPickerIsDescendant(option, path) {
			return true
		}
	}
	return false
}

func (p rootPicker) visibleRows() []setupRow {
	rows := make([]setupRow, 0, len(p.options))
	for _, option := range p.options {
		rows = append(rows, p.visibleRowsFor(option, "", 0)...)
	}
	return rows
}

func (p rootPicker) visiblePaths() []string {
	rows := p.visibleRows()
	paths := make([]string, 0, len(rows))
	for _, row := range rows {
		paths = append(paths, row.Path)
	}
	return paths
}

func (p rootPicker) visibleRowsFor(path, parent string, depth int) []setupRow {
	children, known := p.children[path]
	expanded := p.expanded[path] && len(children) > 0
	checked := p.isPathChecked(path)
	rows := []setupRow{
		{
			Path:       path,
			Label:      rootPickerRowLabel(path, depth),
			Parent:     parent,
			Depth:      depth,
			Checked:    checked,
			Partial:    !checked && p.hasCheckedDescendant(path),
			Expandable: !known || len(children) > 0,
			Expanded:   expanded,
			Count:      p.pathCount(path),
		},
	}
	if !expanded {
		return rows
	}
	for _, child := range children {
		rows = append(rows, p.visibleRowsFor(child, path, depth+1)...)
	}
	return rows
}

func (p rootPicker) knownRows() []setupRow {
	rows := make([]setupRow, 0, len(p.options))
	for _, option := range p.options {
		rows = append(rows, p.knownRowsFor(option, "", 0)...)
	}
	return rows
}

func (p rootPicker) knownRowsFor(path, parent string, depth int) []setupRow {
	rows := []setupRow{{Path: path, Parent: parent, Depth: depth}}
	for _, child := range p.children[path] {
		rows = append(rows, p.knownRowsFor(child, path, depth+1)...)
	}
	return rows
}

func (p rootPicker) selectedRoots() []string {
	roots := make([]string, 0, len(p.checked))
	seen := map[string]bool{}
	for _, row := range p.knownRows() {
		if p.checked[row.Path] {
			roots = append(roots, row.Path)
			seen[row.Path] = true
		}
	}
	extra := make([]string, 0, len(p.checked))
	for path, checked := range p.checked {
		if checked && !seen[path] {
			extra = append(extra, path)
		}
	}
	sort.Strings(extra)
	roots = append(roots, extra...)
	return roots
}

func (p rootPicker) hasCheckedDescendant(path string) bool {
	for checkedPath, checked := range p.checked {
		if checked && rootPickerIsDescendant(path, checkedPath) {
			return true
		}
	}
	return false
}

func (p rootPicker) hasCheckedAncestor(path string) bool {
	for checkedPath, checked := range p.checked {
		if checked && rootPickerIsDescendant(checkedPath, path) {
			return true
		}
	}
	return false
}

func (p rootPicker) isPathChecked(path string) bool {
	return p.checked[path] || p.hasCheckedAncestor(path) || p.hasAllCheckedChildren(path)
}

func (p rootPicker) hasAllCheckedChildren(path string) bool {
	children, known := p.children[path]
	if !known || len(children) == 0 {
		return false
	}
	for _, child := range children {
		if !p.isPathChecked(child) {
			return false
		}
	}
	return true
}

func (p *rootPicker) expandPath(path string) []string {
	if p.expanded == nil {
		p.expanded = map[string]bool{}
	}
	p.ensureChildren(path)
	if len(p.children[path]) > 0 {
		p.expanded[path] = true
	}
	return p.children[path]
}

func (p *rootPicker) collapseOrSelectParent(row setupRow) {
	if p.expanded == nil {
		p.expanded = map[string]bool{}
	}
	if row.Expanded {
		p.expanded[row.Path] = false
		return
	}
	if row.Depth > 0 {
		p.selectPath(row.Parent)
		return
	}
	if row.Expandable {
		p.expanded[row.Path] = false
	}
}

func (p *rootPicker) revealChecked() {
	roots := make([]string, 0, len(p.checked))
	for root, checked := range p.checked {
		if checked {
			roots = append(roots, root)
		}
	}
	sort.Strings(roots)
	for _, root := range roots {
		p.revealPath(root)
	}
}

func (p *rootPicker) revealPath(path string) {
	for _, option := range p.options {
		if !rootPickerIsDescendant(option, path) {
			continue
		}
		current := option
		for current != "" && current != path {
			children := p.ensureChildren(current)
			next := rootPickerDirectChildOnPath(current, path, children)
			if next == "" {
				break
			}
			p.expanded[current] = true
			current = next
		}
		return
	}
}

func (p *rootPicker) ensureChildren(path string) []string {
	if children, ok := p.children[path]; ok {
		return children
	}
	children, _ := discoverRootPickerChildren(path)
	children = mergeCheckedRootPickerChildren(path, children, p.checked)
	if p.children == nil {
		p.children = map[string][]string{}
	}
	p.children[path] = children
	return children
}

func (p rootPicker) pathCount(path string) *int {
	if p.counts == nil {
		return nil
	}
	count, ok := p.counts[path]
	if !ok {
		return nil
	}
	return &count
}

func (p rootPicker) missingCountPaths(paths []string) []string {
	missing := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if p.counts != nil {
			if _, ok := p.counts[path]; ok {
				continue
			}
		}
		missing = append(missing, path)
	}
	return missing
}

func (p *rootPicker) toggleRow(row setupRow) {
	if p.checked == nil {
		p.checked = checkedOnboardingOptions(p.options)
	}
	if !p.checked[row.Path] && p.hasCheckedAncestor(row.Path) {
		p.excludeFromCheckedAncestor(row.Path)
		p.uncheckDescendants(row.Path)
		return
	}
	if !p.checked[row.Path] && p.hasAllCheckedChildren(row.Path) {
		p.uncheckDescendants(row.Path)
		return
	}
	next := !p.checked[row.Path]
	p.checked[row.Path] = next
	if !next {
		return
	}
	if row.Depth == 0 {
		p.uncheckDescendants(row.Path)
		return
	}
	if row.Parent != "" {
		p.uncheckAncestors(row.Path)
		p.uncheckDescendants(row.Path)
	}
}

func (p *rootPicker) excludeFromCheckedAncestor(path string) {
	ancestors := make([]string, 0, len(p.checked))
	for checkedPath, checked := range p.checked {
		if checked && rootPickerIsDescendant(checkedPath, path) {
			ancestors = append(ancestors, checkedPath)
		}
	}
	sort.Slice(ancestors, func(i, j int) bool {
		return len([]rune(ancestors[i])) > len([]rune(ancestors[j]))
	})
	for _, ancestor := range ancestors {
		p.checked[ancestor] = false
		p.checkSiblingsAlongPath(ancestor, path)
	}
}

func (p *rootPicker) checkSiblingsAlongPath(root, excluded string) {
	current := root
	for current != "" && current != excluded {
		children := p.ensureChildren(current)
		next := rootPickerDirectChildOnPath(current, excluded, children)
		for _, child := range children {
			if child != next && child != excluded {
				p.checked[child] = true
			}
		}
		if next == "" {
			return
		}
		current = next
	}
}

func (p *rootPicker) uncheckDescendants(path string) {
	for checkedPath := range p.checked {
		if rootPickerIsDescendant(path, checkedPath) {
			p.checked[checkedPath] = false
		}
	}
}

func (p *rootPicker) uncheckAncestors(path string) {
	for checkedPath := range p.checked {
		if rootPickerIsDescendant(checkedPath, path) {
			p.checked[checkedPath] = false
		}
	}
}

func (p *rootPicker) selectPath(path string) {
	rows := p.visibleRows()
	for index, row := range rows {
		if row.Path == path {
			p.selected = index
			return
		}
	}
}

func (p *rootPicker) addOption(path string) {
	for _, existing := range p.options {
		if existing == path {
			return
		}
	}
	p.options = append(p.options, path)
}

func mergeCheckedRootPickerChildren(root string, children []string, checked map[string]bool) []string {
	seen := make(map[string]bool, len(children))
	merged := make([]string, 0, len(children))
	for _, child := range children {
		seen[child] = true
		merged = append(merged, child)
	}
	extra := make([]string, 0, len(checked))
	for path, isChecked := range checked {
		if !isChecked || !rootPickerIsDirectChild(root, path) || seen[path] {
			continue
		}
		extra = append(extra, path)
	}
	sort.Strings(extra)
	return append(merged, extra...)
}

func rootPickerDirectChildOnPath(parent, target string, children []string) string {
	for _, child := range children {
		if normalizeRootPickerPath(child) == normalizeRootPickerPath(target) || rootPickerIsDescendant(child, target) {
			return child
		}
	}
	return ""
}

func discoverRootPickerChildren(root string) ([]string, error) {
	expanded, err := config.ExpandPath(root)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(expanded)
	if err != nil {
		return nil, err
	}
	children := []string{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		children = append(children, filepath.Join(root, entry.Name()))
	}
	return children, nil
}

func rootPickerRowLabel(path string, depth int) string {
	if depth == 0 {
		return path
	}
	return filepath.Base(path)
}

func rootPickerIsDescendant(parent, child string) bool {
	parent = normalizeRootPickerPath(parent)
	child = normalizeRootPickerPath(child)
	return child != parent && strings.HasPrefix(child, parent+"/")
}

func rootPickerIsDirectChild(parent, child string) bool {
	if !rootPickerIsDescendant(parent, child) {
		return false
	}
	relative := strings.TrimPrefix(normalizeRootPickerPath(child), normalizeRootPickerPath(parent)+"/")
	return !strings.Contains(relative, "/")
}

func normalizeRootPickerPath(value string) string {
	if expanded, err := config.ExpandPath(value); err == nil {
		value = expanded
	}
	return strings.TrimRight(filepath.ToSlash(value), "/")
}

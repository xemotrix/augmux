package tui

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
	"github.com/xemotrix/augmux/internal/core"
	"github.com/xemotrix/augmux/internal/styles"
)

type fileCategory int

const (
	catWorktreeOnly fileCategory = iota // changed only in worktree branch
	catBothClean                        // changed in both, no conflict
	catConflict                         // conflicting
)

type categorizedFile struct {
	path     string
	category fileCategory
}

// conflictTreeState holds the state for the conflict tree overlay.
type conflictTreeState struct {
	agentDesc string
	files     []categorizedFile
	rendered  string // pre-rendered tree string
	scroll    int    // scroll offset (lines from top)
}

// computeConflictTree gathers file change data for the given agent and builds
// the categorized file list. Must be called from a tea.Cmd (does git I/O).
func computeConflictTree(repoRoot string, ag *core.Agent) *conflictTreeState {
	srcBranch := core.SourceBranch(repoRoot)

	mergeBase := core.GitMust(repoRoot, "merge-base", srcBranch, ag.Branch)
	if mergeBase == "" {
		return nil
	}

	agentFiles := splitNonEmpty(core.GitMust(repoRoot, "diff", "--name-only", mergeBase+".."+ag.Branch))
	sourceFiles := splitNonEmpty(core.GitMust(repoRoot, "diff", "--name-only", mergeBase+".."+srcBranch))

	// --name-only outputs conflicted filenames (one per line) in the
	// "Conflicted file info" section of stdout, right after the tree OID.
	mtOut := core.GitMust(repoRoot, "merge-tree", "--write-tree", "--name-only", srcBranch, ag.Branch)
	conflictFiles := parseMergeTreeConflictFiles(mtOut)

	sourceSet := toSet(sourceFiles)
	conflictSet := toSet(conflictFiles)

	var files []categorizedFile
	seen := make(map[string]bool)

	for _, f := range agentFiles {
		if seen[f] {
			continue
		}
		seen[f] = true
		switch {
		case conflictSet[f]:
			files = append(files, categorizedFile{path: f, category: catConflict})
		case sourceSet[f]:
			files = append(files, categorizedFile{path: f, category: catBothClean})
		default:
			files = append(files, categorizedFile{path: f, category: catWorktreeOnly})
		}
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].category != files[j].category {
			return files[i].category > files[j].category // conflicts first
		}
		return files[i].path < files[j].path
	})

	state := &conflictTreeState{
		agentDesc: ag.Description,
		files:     files,
	}
	state.rendered = renderConflictTree(state)
	return state
}

// parseMergeTreeConflictFiles extracts conflict filenames from the stdout of
// "git merge-tree --write-tree --name-only". The output format is:
//
//	<tree OID>
//	<conflicted file 1>
//	<conflicted file 2>
//	...
//	<blank line>
//	<informational messages (CONFLICT ..., Auto-merging ...)>
//
// The first line is always the tree OID. Subsequent non-blank lines before the
// informational messages section are the conflicted filenames.
func parseMergeTreeConflictFiles(stdout string) []string {
	if stdout == "" {
		return nil
	}
	lines := strings.Split(stdout, "\n")
	if len(lines) < 2 {
		return nil
	}

	var files []string
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		files = append(files, line)
	}
	return files
}

// renderConflictTree builds the lipgloss/tree representation.
func renderConflictTree(state *conflictTreeState) string {
	colorForCat := func(cat fileCategory) lipgloss.TerminalColor {
		switch cat {
		case catConflict:
			return styles.ColorRed
		case catBothClean:
			return styles.ColorYellow
		default:
			return styles.ColorGreen
		}
	}

	type dirNode struct {
		name     string
		children []*dirNode
		files    []categorizedFile
	}

	root := &dirNode{name: ""}
	nodeMap := map[string]*dirNode{"": root}

	var ensureDir func(string) *dirNode
	ensureDir = func(dir string) *dirNode {
		if n, ok := nodeMap[dir]; ok {
			return n
		}
		parent := ensureDir(filepath.Dir(dir))
		n := &dirNode{name: filepath.Base(dir)}
		parent.children = append(parent.children, n)
		nodeMap[dir] = n
		return n
	}

	for _, f := range state.files {
		dir := filepath.Dir(f.path)
		if dir == "." {
			dir = ""
		}
		node := ensureDir(dir)
		node.files = append(node.files, f)
	}

	var buildTree func(dn *dirNode) *tree.Tree
	buildTree = func(dn *dirNode) *tree.Tree {
		t := tree.New()
		dirStyle := lipgloss.NewStyle().Foreground(styles.ColorCyan).Bold(true)
		t.Root(dirStyle.Render(dn.name + "/"))

		for _, child := range dn.children {
			t.Child(buildTree(child))
		}
		for _, f := range dn.files {
			fname := filepath.Base(f.path)
			style := lipgloss.NewStyle().Foreground(colorForCat(f.category))
			t.Child(style.Render(fname))
		}

		t.EnumeratorStyle(lipgloss.NewStyle().Foreground(styles.ColorDimGray).PaddingRight(1))
		return t
	}

	rootTree := tree.New()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.ColorAccent)
	rootTree.Root(titleStyle.Render("Changed files — " + state.agentDesc))
	rootTree.EnumeratorStyle(lipgloss.NewStyle().Foreground(styles.ColorDimGray).PaddingRight(1))

	for _, child := range root.children {
		rootTree.Child(buildTree(child))
	}
	for _, f := range root.files {
		fname := filepath.Base(f.path)
		style := lipgloss.NewStyle().Foreground(colorForCat(f.category))
		rootTree.Child(style.Render(fname))
	}

	presentCats := make(map[fileCategory]bool)
	for _, f := range state.files {
		presentCats[f.category] = true
	}

	legendEntries := []string{"", styles.DefaultStyle.Render("Legend:")}
	if presentCats[catConflict] {
		legendEntries = append(legendEntries, lipgloss.NewStyle().Foreground(styles.ColorRed).Render("  ● conflict"))
	}
	if presentCats[catBothClean] {
		legendEntries = append(legendEntries, lipgloss.NewStyle().Foreground(styles.ColorYellow).Render("  ● changed in both (no conflict)"))
	}
	if presentCats[catWorktreeOnly] {
		legendEntries = append(legendEntries, lipgloss.NewStyle().Foreground(styles.ColorGreen).Render("  ● changed only in worktree"))
	}
	legendEntries = append(legendEntries, "", styles.HintStyle.Render("j/k scroll · ctrl-u/ctrl-d half-page · esc close"))
	legend := lipgloss.JoinVertical(lipgloss.Left, legendEntries...)

	return rootTree.String() + "\n" + legend
}

// viewConflictTree renders the scrollable conflict tree view.
func viewConflictTree(state *conflictTreeState, width, height int) string {
	lines := strings.Split(state.rendered, "\n")
	total := len(lines)

	visibleHeight := height - 2
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	maxScroll := total - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if state.scroll > maxScroll {
		state.scroll = maxScroll
	}
	if state.scroll < 0 {
		state.scroll = 0
	}

	end := state.scroll + visibleHeight
	if end > total {
		end = total
	}
	visible := lines[state.scroll:end]

	pad := lipgloss.NewStyle().PaddingLeft(2)
	return pad.Render(strings.Join(visible, "\n"))
}

func splitNonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

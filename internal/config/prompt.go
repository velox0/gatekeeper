package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ServerBlockRef identifies a specific server block within the config.
type ServerBlockRef struct {
	ListenerIdx int
	ServerIdx   int
	Listen      string
	ServerName  string
}

// ListServerBlocks enumerates all server blocks across all listeners.
func ListServerBlocks(cfg *Config) []ServerBlockRef {
	var refs []ServerBlockRef
	for li, ln := range cfg.Listeners {
		for si, srv := range ln.Servers {
			refs = append(refs, ServerBlockRef{
				ListenerIdx: li,
				ServerIdx:   si,
				Listen:      ln.Listen,
				ServerName:  srv.ServerName,
			})
		}
	}
	return refs
}

// PromptServerBlockSelection displays all server blocks and asks the user
// to select one or more. The user can enter selections like "1", "1,3",
// "1-3", or "1,3-5". Returns the selected indices into the refs slice.
//
// If includeGlobal is true, option 0 ("global scope") is also shown.
func PromptServerBlockSelection(refs []ServerBlockRef, includeGlobal bool) (globalSelected bool, selectedRefs []ServerBlockRef) {
	fmt.Println()
	fmt.Println("Available scopes:")
	if includeGlobal {
		fmt.Printf("  [0] global (applies to all server blocks)\n")
	}
	for i, ref := range refs {
		name := ref.ServerName
		if name == "" {
			name = "<default>"
		}
		fmt.Printf("  [%d] %s → %s\n", i+1, ref.Listen, name)
	}
	fmt.Println()
	fmt.Print("Select scope(s) (e.g. 0, 1, 1-3, 1,3): ")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		fmt.Fprintln(os.Stderr, "error: no input")
		os.Exit(1)
	}
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		fmt.Fprintln(os.Stderr, "error: empty selection")
		os.Exit(1)
	}

	indices := parseSelection(input, len(refs), includeGlobal)

	for _, idx := range indices {
		if idx == 0 && includeGlobal {
			globalSelected = true
		} else {
			selectedRefs = append(selectedRefs, refs[idx-1])
		}
	}

	return globalSelected, selectedRefs
}

// parseSelection parses a selection string like "1,3-5" into a sorted,
// deduplicated slice of indices. With includeGlobal=true, 0 is a valid index.
func parseSelection(input string, maxIdx int, includeGlobal bool) []int {
	minValid := 1
	if includeGlobal {
		minValid = 0
	}

	seen := make(map[int]bool)
	var result []int

	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			lo, err1 := strconv.Atoi(strings.TrimSpace(bounds[0]))
			hi, err2 := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err1 != nil || err2 != nil || lo < minValid || hi > maxIdx || lo > hi {
				fmt.Fprintf(os.Stderr, "error: invalid range %q\n", part)
				os.Exit(1)
			}
			for i := lo; i <= hi; i++ {
				if !seen[i] {
					seen[i] = true
					result = append(result, i)
				}
			}
		} else {
			idx, err := strconv.Atoi(part)
			if err != nil || idx < minValid || idx > maxIdx {
				fmt.Fprintf(os.Stderr, "error: invalid selection %q\n", part)
				os.Exit(1)
			}
			if !seen[idx] {
				seen[idx] = true
				result = append(result, idx)
			}
		}
	}

	if len(result) == 0 {
		fmt.Fprintln(os.Stderr, "error: no valid selections")
		os.Exit(1)
	}

	return result
}

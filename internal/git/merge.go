package git

// composeWorkingTree merges staged + unstaged + untracked file diffs into a
// single set. Hunks are tagged by source before merging.
func composeWorkingTree(staged, unstaged, untracked []FileDiff) []FileDiff {
	tagHunks(staged, SourceStaged)
	tagHunks(unstaged, SourceUnstaged)
	result := mergeFileDiffs(staged, unstaged)
	return append(result, untracked...)
}

// composeBranch merges branch (committed) diffs with working tree diffs.
// The working tree components (staged, unstaged, untracked) should already
// be tagged by source before calling this function.
func composeBranch(branch, workingTree []FileDiff) []FileDiff {
	tagHunks(branch, SourceBranch)
	return mergeFileDiffs(branch, workingTree)
}

// tagHunks sets the Source field on all hunks in the given file diffs.
func tagHunks(files []FileDiff, source HunkSource) {
	for i := range files {
		for j := range files[i].Hunks {
			files[i].Hunks[j].Source = source
		}
	}
}

// mergeFileDiffs combines two sets of file diffs.
// For the same path, hunks from both are combined (base first, then overlay).
// New paths are appended. The inputs are not mutated.
// If a file was added in base and deleted in overlay, it is removed entirely
// (net zero change — e.g. file added on branch then deleted in working tree).
func mergeFileDiffs(base, overlay []FileDiff) []FileDiff {
	result := make([]FileDiff, len(base))
	copy(result, base)

	seen := make(map[string]int, len(result))
	baseStatus := make(map[string]FileStatus, len(result))
	for i, f := range result {
		seen[f.Path] = i
		baseStatus[f.Path] = f.Status
	}

	remove := make(map[int]bool)
	for _, f := range overlay {
		if idx, ok := seen[f.Path]; ok {
			if f.Status == StatusDeleted && baseStatus[f.Path] == StatusAdded {
				// Added in base, deleted in overlay — net zero
				remove[idx] = true
			} else {
				// Same file — combine hunks
				combined := make([]Hunk, len(result[idx].Hunks), len(result[idx].Hunks)+len(f.Hunks))
				copy(combined, result[idx].Hunks)
				combined = append(combined, f.Hunks...)
				result[idx].Hunks = combined
			}
		} else {
			result = append(result, f)
			seen[f.Path] = len(result) - 1
		}
	}

	if len(remove) > 0 {
		filtered := make([]FileDiff, 0, len(result)-len(remove))
		for i, f := range result {
			if !remove[i] {
				filtered = append(filtered, f)
			}
		}
		return filtered
	}

	return result
}

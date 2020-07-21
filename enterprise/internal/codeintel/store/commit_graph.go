package store

// TODO - document
type CommitMeta struct {
	Parents []string
	Uploads []UploadMeta
}

// TODO - document
type UploadMeta struct {
	UploadID int
	Root     string
	Indexer  string
	Distance int
}

// TODO - document
func calculateReachability(commitMeta map[string]CommitMeta) map[string][]UploadMeta {
	graph := map[string][]string{}
	for commit, meta := range commitMeta {
		graph[commit] = meta.Parents
	}

	reverseGraph := reverseGraph(graph)
	ordering := topologicalSort(graph)

	allDistances1 := map[string][]UploadMeta{} // TODO - rename
	allDistances2 := map[string][]UploadMeta{}
	for commit, meta := range commitMeta {
		for _, uploadMeta := range meta.Uploads {
			allDistances1[commit] = append(allDistances1[commit], uploadMeta)
			allDistances2[commit] = append(allDistances2[commit], uploadMeta)
		}
	}

	for _, commit := range ordering {
		uploads := allDistances1[commit]
		for _, parent := range reverseGraph[commit] {
			for _, upload := range allDistances1[parent] {
				uploads = addUploadMeta(uploads, upload.UploadID, upload.Root, upload.Indexer, upload.Distance+1)
			}
		}

		allDistances1[commit] = uploads
	}

	for i := len(ordering) - 1; i >= 0; i-- {
		commit := ordering[i]
		uploads := allDistances2[commit]
		for _, parent := range graph[commit] {
			for _, upload := range allDistances2[parent] {
				uploads = addUploadMeta(uploads, upload.UploadID, upload.Root, upload.Indexer, upload.Distance+1)
			}
		}

		allDistances2[commit] = uploads
	}

	for commit, uploads := range allDistances1 {
		for _, upload := range allDistances2[commit] {
			uploads = addUploadMeta(uploads, upload.UploadID, upload.Root, upload.Indexer, upload.Distance)
		}

		allDistances1[commit] = uploads
	}

	return allDistances1
}

// TODO - document
func addUploadMeta(things []UploadMeta, uploadID int, root, indexer string, distance int) []UploadMeta {
	for i, x := range things {
		if root == x.Root && indexer == x.Indexer {
			if distance < x.Distance || (distance == x.Distance && uploadID < x.UploadID) {
				things[i].UploadID = uploadID
				things[i].Distance = distance
			}

			return things
		}
	}

	return append(things, UploadMeta{
		UploadID: uploadID,
		Root:     root,
		Indexer:  indexer,
		Distance: distance,
	})
}

// TODO - document
func reverseGraph(graph map[string][]string) map[string][]string {
	reverse := map[string][]string{}
	for child := range graph {
		reverse[child] = nil
	}

	for child, parents := range graph {
		for _, parent := range parents {
			reverse[parent] = append(reverse[parent], child)
		}
	}

	return reverse
}

type SortMarker int

const (
	MarkTemp SortMarker = iota
	MarkPermenant
)

// TODO - document
func topologicalSort(graph map[string][]string) []string {
	mark := map[string]SortMarker{}
	i := len(graph)
	ordering := make([]string, len(graph))

	// TODO - do iteratively a way to do non-recursively

	var visit func(commit string)
	visit = func(commit string) {
		if mark, ok := mark[commit]; ok {
			if mark == MarkTemp {
				panic("cycles") // TODO - don't panic, return an error
			}

			return
		}

		parents, ok := graph[commit]
		if !ok {
			// panic("malformed graph") // TODO - return an error
			return
		}

		mark[commit] = MarkTemp

		for _, parent := range parents {
			visit(parent)
		}

		mark[commit] = MarkPermenant
		i--
		ordering[i] = commit
	}

	for v := range graph {
		visit(v)
	}

	return ordering
}

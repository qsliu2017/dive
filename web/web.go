package web

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/wagoodman/dive/dive/filetree"
	"github.com/wagoodman/dive/dive/image"
)

//go:generate bun run build

//go:embed dist/*
var assets embed.FS

func Run(port int, imageName string, analysis *image.AnalysisResult, treeStack filetree.Comparer) error {
	dist, err := fs.Sub(assets, "dist")
	if err != nil {
		return errors.Join(err, errors.New("cannot find dist directory"))
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(dist)))
	mux.HandleFunc("/api/analysis", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(analysis); err != nil {
			log.Default().Printf("cannot encode analysis: %v", err)
		}
	})

	layerMap := make(map[string]*layer)
	for _, l := range analysis.Layers {
		layerMap[l.Id] = newLayer(l)
	}
	layerList := make([]*layer, 0, len(analysis.Layers))
	for _, l := range analysis.Layers {
		layerList = append(layerList, newLayer(l))
	}
	// /api/layer?id
	// /api/layer
	mux.HandleFunc("/api/layer", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id != "" {
			layer, has := layerMap[id]
			if !has {
				http.Error(w, fmt.Sprintf("layer id not found: %s", id), http.StatusNotFound)
				return
			}
			if err := json.NewEncoder(w).Encode(layer); err != nil {
				log.Default().Printf("cannot encode layer: %v", err)
			}
			return
		}
		if err := json.NewEncoder(w).Encode(layerList); err != nil {
			log.Default().Printf("cannot encode layer list: %v", err)
		}
	})

	treeMap := make(map[uuid.UUID]map[string]*fileNode)
	for _, tree := range analysis.RefTrees {
		nodeMap := make(map[string]*fileNode)
		tree.Root.VisitDepthChildFirst(
			func(fn *filetree.FileNode) error {
				nodeMap[fn.Path()] = newFileNode(fn)
				return nil
			},
			nil,
			filetree.GetSortOrderStrategy(filetree.ByName),
		)
		treeMap[tree.Id] = nodeMap
	}
	// /api/filetree?id
	// /api/filetree
	mux.HandleFunc("/api/filetree", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id != "" {
			tree, has := treeMap[uuid.MustParse(id)]
			if !has {
				http.Error(w, fmt.Sprintf("file tree id not found: %s", id), http.StatusNotFound)
				return
			}
			if err := json.NewEncoder(w).Encode(tree); err != nil {
				log.Default().Printf("cannot encode file tree: %v", err)
			}
			return
		}
		if err := json.NewEncoder(w).Encode(treeMap); err != nil {
			log.Default().Printf("cannot encode file tree list: %v", err)
		}
	})

	return http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}

type layer struct {
	Id      string
	Index   int
	Command string
	Size    uint64
	TreeId  uuid.UUID
	Names   []string
	Digest  string
}

func newLayer(l *image.Layer) *layer {
	return &layer{
		Id:      l.Id,
		Index:   l.Index,
		Command: l.Command,
		Size:    l.Size,
		TreeId:  l.Tree.Id,
		Names:   l.Names,
		Digest:  l.Digest,
	}
}

type fileTree struct {
	Size      int
	FileSize  uint64
	Name      string
	Id        uuid.UUID
	SortOrder filetree.SortOrder
	ParentId  uuid.UUID
	Children  map[string]uuid.UUID
	Path      string
}

func newFileTree(tree *filetree.FileTree) *fileTree {
	children := make(map[string]uuid.UUID)
	for id, child := range tree.Root.Children {
		children[id] = child.Tree.Id
	}
	parentId := uuid.Nil
	if tree.Root.Parent != nil {
		parentId = tree.Root.Parent.Tree.Id
	}
	return &fileTree{
		Size:      tree.Size,
		FileSize:  tree.FileSize,
		Name:      tree.Name,
		Id:        tree.Id,
		SortOrder: tree.SortOrder,
		ParentId:  parentId,
		Children:  children,
		Path:      tree.Root.Path(),
	}
}

type fileNode struct {
	Size     int64
	Name     string
	Children []string
	Path     string
}

func newFileNode(fn *filetree.FileNode) *fileNode {
	children := make([]string, 0, len(fn.Children))
	for child := range fn.Children {
		children = append(children, child)
	}
	return &fileNode{
		Size:     fn.Size,
		Name:     fn.Name,
		Children: children,
		Path:     fn.Path(),
	}
}

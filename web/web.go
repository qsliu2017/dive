package web

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/wagoodman/dive/dive/filetree"
	"github.com/wagoodman/dive/dive/image"
)

//go:generate bun i
//go:generate bun run build

//go:embed dist/*
var assets embed.FS

func Run(port int, imageName string, analysis *image.AnalysisResult, treeStack filetree.Comparer) error {
	dist, err := fs.Sub(assets, "dist")
	if err != nil {
		return errors.Join(err, errors.New("cannot find dist directory"))
	}

	e := echo.New()

	e.StaticFS("/", dist)
	e.GET("/api/analysis", func(c echo.Context) error {
		return c.JSON(http.StatusOK, analysis)
	})

	layerIdList := make([]string, 0, len(analysis.Layers))
	layerMap := make(map[string]*layer)
	for _, l := range analysis.Layers {
		layerIdList = append(layerIdList, l.Id)
		layerMap[l.Id] = newLayer(l)
	}

	e.GET("/api/layer", func(c echo.Context) error {
		return c.JSON(http.StatusOK, layerIdList)
	})
	e.GET("/api/layer/:id", func(c echo.Context) error {
		id := c.Param("id")
		layer, has := layerMap[id]
		if !has {
			return c.JSON(http.StatusNotFound, fmt.Sprintf("layer id not found: %s", id))
		}
		return c.JSON(http.StatusOK, layer)
	})

	treeIdList := make([]uuid.UUID, 0, len(analysis.RefTrees))
	treeMap := make(map[uuid.UUID]map[string]*fileNode)
	for _, tree := range analysis.RefTrees {
		treeIdList = append(treeIdList, tree.Id)
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
	e.GET("/api/tree", func(c echo.Context) error {
		return c.JSON(http.StatusOK, treeIdList)
	})
	e.GET("/api/tree/:tree-id", func(c echo.Context) error {
		id := c.Param("tree-id")
		tree, has := treeMap[uuid.MustParse(id)]
		if !has {
			return c.JSON(http.StatusNotFound, fmt.Sprintf("file tree id not found: %s", id))
		}
		return c.JSON(http.StatusOK, tree)
	})
	e.GET("/api/tree/:tree-id/:path", func(c echo.Context) error {
		treeId := c.Param("tree-id")
		path := "/" + c.Param("path")
		tree, has := treeMap[uuid.MustParse(treeId)]
		if !has {
			return c.JSON(http.StatusNotFound, fmt.Sprintf("file tree id not found: %s", treeId))
		}
		node, has := tree[path]
		if !has {
			return c.JSON(http.StatusNotFound, fmt.Sprintf("file node path not found: %s", path))
		}
		return c.JSON(http.StatusOK, node)
	})

	return e.Start(fmt.Sprintf(":%d", port))
}

type layer struct {
	Id      string    `json:"id"`
	Index   int       `json:"index"`
	Command string    `json:"command"`
	Size    uint64    `json:"size"`
	TreeId  uuid.UUID `json:"treeId"`
	Names   []string  `json:"names"`
	Digest  string    `json:"digest"`
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
	Info     *filetree.FileInfo
	DiffType string
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
		Info:     &fn.Data.FileInfo,
		DiffType: fn.Data.DiffType.String(),
	}
}

package assets

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type AssetLoaderFunc func(name string) Asset

type Loader interface {
	Load(name string) Asset
	LoadFromSha256(sha string) (Asset, error)
}

type impl struct {
	basePath string

	path2sha map[string]string
	assets   map[string]Asset

	mu sync.Mutex
}

type LoaderOptionFunc func(*impl)

func BasePath(base string) LoaderOptionFunc {
	return func(loader *impl) {
		loader.basePath = base
	}
}

func NewLoader(ctx context.Context, opts ...LoaderOptionFunc) (Loader, error) {
	loader := &impl{
		basePath: ".",
		path2sha: map[string]string{},
		assets:   map[string]Asset{},
	}

	for _, opt := range opts {
		opt(loader)
	}

	ignoredFiles := map[string]struct{}{
		".DS_Store": {},
	}

	filepath.Walk(loader.basePath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			_, ignored := ignoredFiles[info.Name()]

			if !ignored && !info.IsDir() {
				loader.Load(
					strings.TrimPrefix(path, loader.basePath),
				)
			}

			return nil
		})

	return loader, nil
}

func (l *impl) Load(path string) Asset {
	l.mu.Lock()
	defer l.mu.Unlock()

	if sha, ok := l.path2sha[path]; ok {
		return l.assets[sha]
	}

	fullPath := l.basePath + path

	assetFile, err := os.Open(fullPath)
	if err != nil {
		panic("failed to open asset " + fullPath + " (" + err.Error() + ")")
	}
	defer assetFile.Close()

	assetContents, err := io.ReadAll(assetFile)
	if err != nil {
		panic("failed to read from asset file " + fullPath + " (" + err.Error() + ")")
	}

	sha256 := fmt.Sprintf("%x", sha256.Sum256(assetContents))
	l.path2sha[path] = sha256

	pathTokens := strings.Split(path, "/")
	assetFileName := pathTokens[len(pathTokens)-1]

	a := &asset{
		path:        fmt.Sprintf("/assets/%s/%s", sha256, assetFileName),
		contentType: contentTypeFromFileName(path),
		body:        assetContents,
		sha256:      sha256,
	}

	l.assets[sha256] = a

	fmt.Println("loaded", path, "as", a.path)

	return a
}

func (l *impl) LoadFromSha256(sha string) (Asset, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if a, ok := l.assets[sha]; ok {
		return a, nil
	}

	return nil, ErrNotFound
}

package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ImgExtWebp = "webp"
	ImgExtSvg  = "svg"
	VidExtWebm = "webm"
)

var (
	win1252Re   = regexp.MustCompile(`%u([0-9a-fA-F]{4})|%([0-9a-fA-F]{2})`)
	altSuffixRe = regexp.MustCompile(` \(\d\)$`)
)

type imgDimensions struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Type   string `json:"type"`
}

type tagEntry struct {
	Img           string         `json:"img"`
	Vid           string         `json:"vid"`
	Ignore        bool           `json:"ignore"`
	Alt           bool           `json:"alt"`
	ImgDimensions *imgDimensions `json:"imgDimensions,omitempty"`
	Aliases       []string       `json:"aliases"`
	StashID       string         `json:"stashID,omitempty"`
}

type statsExport struct {
	GeneratedAt  string `json:"generated_at"`
	StashDBTotal int    `json:"stashdb_total"`
	Total        int    `json:"total"`
	Both         int    `json:"both"`
	Img          int    `json:"img"`
	Vid          int    `json:"vid"`
}

func cleanFileName(filename string) string {
	s := strings.TrimSpace(filename)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '.':
			// skip
		case ':':
			b.WriteByte('-')
		case ' ', '/', '\\':
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	s = win1252Re.ReplaceAllStringFunc(b.String(), func(m string) string {
		if strings.HasPrefix(m, "%u") {
			n, _ := strconv.ParseUint(m[2:], 16, 32)
			return string(rune(n))
		}
		n, _ := strconv.ParseUint(m[1:], 16, 8)
		return string(byte(n))
	})
	return s
}

func pathToExportFilename(path, tagName string) string {
	if path == "" {
		return ""
	}
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	return cleanFileName(tagName) + "." + ext
}

func saniTagExports(inv map[string]*tagEntry) {
	for tagName, v := range inv {
		v.Img = pathToExportFilename(v.Img, tagName)
		v.Vid = pathToExportFilename(v.Vid, tagName)
	}
}

// fileIndex maps filename -> full path.
type fileIndex map[string]string

func buildFileIndex(tagPath string) (fileIndex, error) {
	entries, err := os.ReadDir(tagPath)
	if err != nil {
		return nil, err
	}
	idx := make(fileIndex)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		idx[name] = filepath.Join(tagPath, name)
	}
	return idx, nil
}

func (idx fileIndex) find(baseName string, exts ...string) string {
	for _, ext := range exts {
		if p, ok := idx[baseName+"."+ext]; ok {
			return p
		}
	}
	return ""
}

func getAltFiles(tagPath string) map[string]bool {
	altDir := filepath.Join(tagPath, "alt")
	entries, err := os.ReadDir(altDir)
	if err != nil {
		return nil
	}
	out := make(map[string]bool)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		base := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		base = altSuffixRe.ReplaceAllString(base, "")
		base = cleanFileName(base) // normalize to match tag fileName
		out[base] = true
	}
	return out
}

func RunSync(cfg *Config) (map[string]*tagEntry, error) {
	stash := newStashClient(cfg.StashURL, cfg.StashAPIKey)
	stashdb := newStashDBClient(cfg.StashDBAPIKey)

	tags, err := stash.getAllTags()
	if err != nil {
		return nil, fmt.Errorf("get tags: %w", err)
	}

	altFiles := getAltFiles(cfg.TagPath)
	fileIdx, err := buildFileIndex(cfg.TagPath)
	if err != nil {
		return nil, fmt.Errorf("get files: %w", err)
	}
	usedFiles := make(map[string]bool)

	inv := make(map[string]*tagEntry)
	var dimTasks []struct {
		tagName string
		path    string
	}
	for _, tag := range tags {
		tagName := tag.Name
		fileName := cleanFileName(tagName)

		var stashID string
		for _, s := range tag.StashIDs {
			if s.Endpoint == StashDBURL {
				stashID = s.StashID
				break
			}
		}

		imgPath := fileIdx.find(fileName, ImgExtWebp, ImgExtSvg)
		vidPath := fileIdx.find(fileName, VidExtWebm)

		for _, f := range []string{imgPath, vidPath} {
			if f != "" {
				usedFiles[filepath.Base(f)] = true
			}
		}

		ignore := tag.IgnoreAutoTag
		for _, p := range ExcludePrefixes {
			if strings.HasPrefix(tagName, p) {
				ignore = true
				break
			}
		}

		if !ignore && stashID == "" {
			fmt.Fprintf(os.Stderr, "No stashID found for tag: %s\n", tagName)
		}

		if imgPath != "" {
			dimTasks = append(dimTasks, struct {
				tagName string
				path    string
			}{tagName, imgPath})
		}

		entry := &tagEntry{
			Img:           imgPath,
			Vid:           vidPath,
			Ignore:        ignore,
			Alt:           altFiles[fileName],
			ImgDimensions: nil,
			Aliases:       tag.Aliases,
			StashID:       stashID,
		}
		inv[tagName] = entry
	}

	// Parallel image dimension lookup
	const maxDimWorkers = 8
	sem := make(chan struct{}, maxDimWorkers)
	var wg sync.WaitGroup
	type dimResult struct {
		tagName string
		dims    *imgDimensions
	}
	results := make(chan dimResult, len(dimTasks))
	for _, t := range dimTasks {
		wg.Add(1)
		go func(tagName, path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			w, h, fmtType, err := GetImageDimensionsWithType(path)
			var d *imgDimensions
			if err == nil && (w > 0 || h > 0 || fmtType == ImgExtSvg) {
				d = &imgDimensions{Width: w, Height: h, Type: fmtType}
			}
			results <- dimResult{tagName, d}
		}(t.tagName, t.path)
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	for r := range results {
		if r.dims != nil {
			inv[r.tagName].ImgDimensions = r.dims
		}
	}

	saniTagExports(inv)

	if err := os.MkdirAll(filepath.Dir(cfg.TagExportPath), 0755); err != nil {
		return nil, err
	}
	tagJSON, _ := json.Marshal(inv)
	if err := os.WriteFile(cfg.TagExportPath, tagJSON, 0644); err != nil {
		return nil, err
	}

	eligible, both, imgOnly, vidOnly := 0, 0, 0, 0
	for _, e := range inv {
		if e.Ignore {
			continue
		}
		eligible++
		hasImg := e.Img != ""
		hasVid := e.Vid != ""
		if hasImg && hasVid {
			both++
		} else if hasImg {
			imgOnly++
		} else if hasVid {
			vidOnly++
		}
	}

	stashdbTotal := 0
	if cfg.StashDBAPIKey != "" {
		stashdbTotal, _ = stashdb.getTagCount()
	}

	stats := statsExport{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		StashDBTotal: stashdbTotal,
		Total:        eligible,
		Both:         both,
		Img:          imgOnly,
		Vid:          vidOnly,
	}
	statsJSON, _ := json.MarshalIndent(stats, "", "  ")
	if err := os.WriteFile(cfg.StatsExportPath, statsJSON, 0644); err != nil {
		return nil, err
	}

	for name := range fileIdx {
		if !usedFiles[name] {
			fmt.Println("Extra file found:", name)
		}
	}

	var noStashIDs []string
	for k, v := range inv {
		if !v.Ignore && v.StashID == "" {
			noStashIDs = append(noStashIDs, k)
		}
	}
	if len(noStashIDs) > 0 {
		fmt.Println("Tags without stashIDs:", noStashIDs)
	}

	return inv, nil
}

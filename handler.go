package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type Handler struct {
	mountPoints []MountPoint
}

func NewHandler(cfg Config) *Handler {
	return &Handler{
		mountPoints: cfg.MountPoints,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/":
		http.Redirect(w, r, "/files", http.StatusMovedPermanently)
	case r.Method == http.MethodGet && r.URL.Path == "/files":
		http.ServeFile(w, r, "./frontend/public/files.html")
	case r.Method == http.MethodGet && r.URL.Path == "/api/files":
		h.handleListFiles(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/mkdir":
		h.handleMkdir(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/move":
		h.handleMove(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/delete":
		h.handleDelete(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/rmdir":
		h.handleRmdir(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/download/"):
		h.handleDownload(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/upload"):
		h.handleUpload(w, r)
	case r.Method == http.MethodGet && (strings.HasPrefix(r.URL.Path, "/css/") || strings.HasPrefix(r.URL.Path, "/js/")):
		http.ServeFile(w, r, "./frontend/public"+r.URL.Path)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleListFiles(w http.ResponseWriter, r *http.Request) {
	if len(h.mountPoints) == 0 {
		http.Error(w, "no mount points", http.StatusInternalServerError)
		return
	}
	mp := h.mountPoints[0]
	absMP, err := filepath.Abs(mp.Path)
	if err != nil {
		http.Error(w, "invalid mount path", http.StatusInternalServerError)
		return
	}

	listPath := r.URL.Query().Get("path")
	targetDir := absMP
	if listPath != "" {
		targetDir = filepath.Join(absMP, listPath)
	}
	if targetDir, err = filepath.Abs(targetDir); err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(targetDir, absMP) {
		http.Error(w, "path outside mount point", http.StatusForbidden)
		return
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		http.Error(w, "could not read directory", http.StatusInternalServerError)
		return
	}

	type fileInfo struct {
		Name    string `json:"name"`
		SavedAs string `json:"saved_as"`
		Size    int64  `json:"size"`
		IsDir   bool   `json:"is_dir"`
	}
	files := make([]fileInfo, 0)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".meta.json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}

		if e.IsDir() {
			files = append(files, fileInfo{
				Name:    e.Name(),
				SavedAs: e.Name(),
				Size:    info.Size(),
				IsDir:   true,
			})
			continue
		}

		name := e.Name()
		savedAs := name
		metaPath := filepath.Join(targetDir, e.Name()+".meta.json")
		metaData, err := os.ReadFile(metaPath)
		if err == nil {
			var meta struct {
				OriginalName string `json:"original_name"`
			}
			if err := json.Unmarshal(metaData, &meta); err == nil && meta.OriginalName != "" {
				name = meta.OriginalName
				savedAs = e.Name()
			}
		}

		files = append(files, fileInfo{
			Name:    name,
			SavedAs: savedAs,
			Size:    info.Size(),
			IsDir:   false,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func (h *Handler) handleMkdir(w http.ResponseWriter, r *http.Request) {
	if len(h.mountPoints) == 0 {
		http.Error(w, "no mount points", http.StatusInternalServerError)
		return
	}
	mp := h.mountPoints[0]
	absMP, err := filepath.Abs(mp.Path)
	if err != nil {
		http.Error(w, "invalid mount path", http.StatusInternalServerError)
		return
	}

	name := r.FormValue("name")
	if name == "" || strings.Contains(name, "\\") || strings.Contains(name, "/") || strings.HasPrefix(name, ".") {
		http.Error(w, "invalid folder name", http.StatusBadRequest)
		return
	}

	dirPath := r.FormValue("path")
	targetDir := absMP
	if dirPath != "" {
		targetDir = filepath.Join(absMP, dirPath)
	}
	if targetDir, err = filepath.Abs(targetDir); err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(targetDir, absMP) {
		http.Error(w, "path outside mount point", http.StatusForbidden)
		return
	}

	fullPath := filepath.Join(targetDir, name)
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		log.Printf("mkdir error: %v", err)
		http.Error(w, "could not create folder", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"path": fullPath})
}

func (h *Handler) handleMove(w http.ResponseWriter, r *http.Request) {
	if len(h.mountPoints) == 0 {
		http.Error(w, "no mount points", http.StatusInternalServerError)
		return
	}
	mp := h.mountPoints[0]
	absMP, err := filepath.Abs(mp.Path)
	if err != nil {
		http.Error(w, "invalid mount path", http.StatusInternalServerError)
		return
	}

	savedName := r.FormValue("saved_as")
	if savedName == "" || strings.Contains(savedName, "\\") || strings.Contains(savedName, "/") || strings.HasPrefix(savedName, ".") {
		http.Error(w, "invalid source", http.StatusBadRequest)
		return
	}

	srcDir := r.FormValue("src_path")
	srcPath := filepath.Join(absMP, srcDir, savedName)
	if srcPath, err = filepath.Abs(srcPath); err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(srcPath, absMP) {
		http.Error(w, "path outside mount point", http.StatusForbidden)
		return
	}

	dstDir := r.FormValue("dst_path")
	dstPath := filepath.Join(absMP, dstDir, savedName)
	if dstPath, err = filepath.Abs(dstPath); err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(dstPath, absMP) {
		http.Error(w, "path outside mount point", http.StatusForbidden)
		return
	}

	os.MkdirAll(filepath.Dir(dstPath), 0755)

	if err := os.Rename(srcPath, dstPath); err != nil {
		log.Printf("move error: %v", err)
		http.Error(w, "could not move file", http.StatusInternalServerError)
		return
	}

	metaSrc := srcPath + ".meta.json"
	metaDst := dstPath + ".meta.json"
	metaData, err := os.ReadFile(metaSrc)
	if err == nil {
		_ = os.WriteFile(metaDst, metaData, 0644)
		_ = os.Remove(metaSrc)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	if len(h.mountPoints) == 0 {
		http.Error(w, "no mount points", http.StatusInternalServerError)
		return
	}
	mp := h.mountPoints[0]
	absMP, err := filepath.Abs(mp.Path)
	if err != nil {
		http.Error(w, "invalid mount path", http.StatusInternalServerError)
		return
	}

	savedName := r.FormValue("saved_as")
	if savedName == "" || strings.Contains(savedName, "\\") || strings.Contains(savedName, "/") || strings.HasPrefix(savedName, ".") {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	filePath := r.FormValue("path")
	targetPath := filepath.Join(absMP, filePath, savedName)
	if targetPath, err = filepath.Abs(targetPath); err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(targetPath, absMP) {
		http.Error(w, "path outside mount point", http.StatusForbidden)
		return
	}

	os.Remove(targetPath + ".meta.json")
	if err := os.Remove(targetPath); err != nil {
		log.Printf("delete error: %v", err)
		http.Error(w, "could not delete file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) handleRmdir(w http.ResponseWriter, r *http.Request) {
	if len(h.mountPoints) == 0 {
		http.Error(w, "no mount points", http.StatusInternalServerError)
		return
	}
	mp := h.mountPoints[0]
	absMP, err := filepath.Abs(mp.Path)
	if err != nil {
		http.Error(w, "invalid mount path", http.StatusInternalServerError)
		return
	}

	folderName := r.FormValue("name")
	if folderName == "" || strings.Contains(folderName, "\\") || strings.Contains(folderName, "/") || strings.HasPrefix(folderName, ".") {
		http.Error(w, "invalid folder name", http.StatusBadRequest)
		return
	}

	dirPath := r.FormValue("path")
	targetDir := filepath.Join(absMP, dirPath, folderName)
	if targetDir, err = filepath.Abs(targetDir); err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(targetDir, absMP) {
		http.Error(w, "path outside mount point", http.StatusForbidden)
		return
	}

	if err := os.RemoveAll(targetDir); err != nil {
		log.Printf("rmdir error: %v", err)
		http.Error(w, "could not delete folder", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) handleDownload(w http.ResponseWriter, r *http.Request) {
	if len(h.mountPoints) == 0 {
		http.Error(w, "no mount points", http.StatusInternalServerError)
		return
	}
	mp := h.mountPoints[0]
	absMP, err := filepath.Abs(mp.Path)
	if err != nil {
		http.Error(w, "invalid mount path", http.StatusInternalServerError)
		return
	}

	filename := strings.TrimPrefix(r.URL.Path, "/download/")
	if filename == "" || strings.Contains(filename, "\\") || strings.Contains(filename, "/") || strings.HasPrefix(filename, ".") {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	destPath := filepath.Join(absMP, filename)
	if destPath, err = filepath.Abs(destPath); err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(destPath, absMP) {
		http.Error(w, "path outside mount point", http.StatusForbidden)
		return
	}

	displayName := filename
	metaPath := destPath + ".meta.json"
	metaData, err := os.ReadFile(metaPath)
	if err == nil {
		var meta struct {
			OriginalName string `json:"original_name"`
		}
		if err := json.Unmarshal(metaData, &meta); err == nil && meta.OriginalName != "" {
			displayName = meta.OriginalName
		}
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+displayName)
	http.ServeFile(w, r, destPath)
}

func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, int64(100*1024*1024))

	err := r.ParseMultipartForm(100 << 20)
	if err != nil {
		http.Error(w, "file too large", http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	filename := filepath.Base(header.Filename)
	if filename == "." || filename == "" {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	// Use the first mount point for uploads
	if len(h.mountPoints) == 0 {
		http.Error(w, "no mount points configured", http.StatusInternalServerError)
		return
	}
	mp := h.mountPoints[0]
	absMP, err := filepath.Abs(mp.Path)
	if err != nil {
		http.Error(w, "invalid mount path", http.StatusInternalServerError)
		return
	}

	// Sanitize: ensure file stays within mount point
	destDir := absMP
	if dir := r.FormValue("dir"); dir != "" {
		safe := filepath.Clean(dir)
		if !filepath.IsAbs(safe) {
			destDir = filepath.Join(absMP, safe)
		}
	}

	if destDir, err = filepath.Abs(destDir); err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(destDir, absMP) {
		http.Error(w, "path outside mount point", http.StatusForbidden)
		return
	}

	os.MkdirAll(destDir, 0755)

	uniqueName := uuid.New().String()[:8] + "-" + filename
	destPath := filepath.Join(destDir, uniqueName)

	metaData, _ := json.Marshal(map[string]string{"original_name": filename})
	os.WriteFile(destPath+".meta.json", metaData, 0644)

	dst, err := os.Create(destPath)
	if err != nil {
		log.Printf("create file error: %v", err)
		http.Error(w, "could not create file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	n, err := io.Copy(dst, file)
	if err != nil {
		log.Printf("write file error: %v", err)
		http.Error(w, "could not save file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"filename": filename,
		"saved_as": uniqueName,
		"path":     destPath,
		"size":     n,
	})
}



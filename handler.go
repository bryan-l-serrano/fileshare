package main

import (
	"encoding/json"
	"fmt"
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
	log.Printf("REQUEST %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
	var status int
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/":
		http.Redirect(w, r, "/files", http.StatusMovedPermanently)
		status = http.StatusMovedPermanently
	case r.Method == http.MethodGet && r.URL.Path == "/files":
		http.ServeFile(w, r, "./frontend/public/files.html")
		status = http.StatusOK
	case r.Method == http.MethodGet && r.URL.Path == "/api/files":
		h.handleListFiles(w, r)
		status = 200
	case r.Method == http.MethodPost && r.URL.Path == "/api/mkdir":
		h.handleMkdir(w, r)
		status = 200
	case r.Method == http.MethodPost && r.URL.Path == "/api/move":
		h.handleMove(w, r)
		status = 200
	case r.Method == http.MethodPost && r.URL.Path == "/api/delete":
		h.handleDelete(w, r)
		status = 200
	case r.Method == http.MethodPost && r.URL.Path == "/api/rmdir":
		h.handleRmdir(w, r)
		status = 200
	case r.Method == http.MethodPost && r.URL.Path == "/api/rename":
		h.handleRename(w, r)
		status = 200
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/download/"):
		h.handleDownload(w, r)
		status = 200
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/upload"):
		h.handleUpload(w, r)
		status = 200
	case r.Method == http.MethodGet && (strings.HasPrefix(r.URL.Path, "/css/") || strings.HasPrefix(r.URL.Path, "/js/")):
		http.ServeFile(w, r, "./frontend/public"+r.URL.Path)
		status = http.StatusOK
	default:
		http.NotFound(w, r)
		status = http.StatusNotFound
	}
	log.Printf("RESPONSE %s %s status=%d", r.Method, r.URL.Path, status)
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
	metaData, readErr := os.ReadFile(metaSrc)
	if readErr == nil {
		var meta struct {
			OriginalName string `json:"original_name"`
			Dir          string `json:"dir"`
		}
		json.Unmarshal(metaData, &meta)
		relDir, _ := filepath.Rel(absMP, filepath.Dir(dstPath))
		if relDir == "." || relDir == "" {
			relDir = ""
		}
		meta.Dir = relDir
		newMeta, _ := json.Marshal(meta)
		os.WriteFile(metaDst, newMeta, 0644)
		os.Remove(metaSrc)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) handleRename(w http.ResponseWriter, r *http.Request) {
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

	newName := r.FormValue("new_name")
	if newName == "" || strings.Contains(newName, "\\") || strings.Contains(newName, "/") || strings.HasPrefix(newName, ".") {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}

	filePath := r.FormValue("path")
	srcPath := filepath.Join(absMP, filePath, savedName)
	if srcPath, err = filepath.Abs(srcPath); err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(srcPath, absMP) {
		http.Error(w, "path outside mount point", http.StatusForbidden)
		return
	}

	dstPath := filepath.Join(absMP, filePath, newName)
	if dstPath, err = filepath.Abs(dstPath); err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(dstPath, absMP) {
		http.Error(w, "path outside mount point", http.StatusForbidden)
		return
	}

	if err := os.Rename(srcPath, dstPath); err != nil {
		log.Printf("rename error: %v", err)
		http.Error(w, "could not rename", http.StatusInternalServerError)
		return
	}

	metaSrc := srcPath + ".meta.json"
	metaDst := dstPath + ".meta.json"
	metaData, err := os.ReadFile(metaSrc)
	if err == nil {
		os.WriteFile(metaDst, metaData, 0644)
		os.Remove(metaSrc)
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
		log.Printf("DOWNLOAD error: no mount points, url=%s", r.URL.Path)
		http.Error(w, "no mount points", http.StatusInternalServerError)
		return
	}
	mp := h.mountPoints[0]
	absMP, err := filepath.Abs(mp.Path)
	if err != nil {
		log.Printf("DOWNLOAD error: abs mp=%v, url=%s", err, r.URL.Path)
		http.Error(w, "invalid mount path", http.StatusInternalServerError)
		return
	}

	filename := strings.TrimPrefix(r.URL.Path, "/download/")
	log.Printf("DOWNLOAD: requested filename=%s absMP=%s", filename, absMP)
	if filename == "" || strings.Contains(filename, "\\") || strings.Contains(filename, "/") || strings.HasPrefix(filename, ".") {
		log.Printf("DOWNLOAD error: invalid filename=%s", filename)
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	metaSearch := filename + ".meta.json"
	destPath := ""
	displayName := filename
	filepath.WalkDir(absMP, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == metaSearch {
			metaData, metaErr := os.ReadFile(path)
			if metaErr != nil {
				return nil
			}
			var meta struct {
				OriginalName string `json:"original_name"`
				Dir          string `json:"dir"`
			}
			if json.Unmarshal(metaData, &meta) == nil {
				displayName = meta.OriginalName
				if displayName == "" {
					displayName = filename
				}
			}
			destPath = filepath.Join(filepath.Dir(path), filename)
			return fmt.Errorf("found")
		}
		return nil
	})
	if destPath == "" {
		log.Printf("DOWNLOAD error: file not found filename=%s", filename)
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	destPath, err = filepath.Abs(destPath)
	if err != nil {
		log.Printf("DOWNLOAD error: abs path=%v, url=%s", err, r.URL.Path)
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	log.Printf("DOWNLOAD: resolved destPath=%s exists=%v", destPath, fileExists(destPath))
	if !strings.HasPrefix(destPath, absMP) {
		http.Error(w, "path outside mount point", http.StatusForbidden)
		return
	}

	if displayName == "" {
		displayName = filename
	}

	f, err := os.Open(destPath)
	if err != nil {
		log.Printf("DOWNLOAD error: os.Open(%s) failed: %v", destPath, err)
		http.Error(w, "could not open file", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		log.Printf("DOWNLOAD error: read failed: %v", err)
		http.Error(w, "could not read file", http.StatusInternalServerError)
		return
	}
	log.Printf("DOWNLOAD: success size=%d displayName=%s", len(data), displayName)
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+displayName)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(data)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
		log.Printf("UPLOAD: dir=%s clean=%s isAbs=%v", dir, safe, filepath.IsAbs(safe))
		if !filepath.IsAbs(safe) {
			destDir = filepath.Join(absMP, safe)
		}
	}

	if destDir, err = filepath.Abs(destDir); err != nil {
		log.Printf("UPLOAD error: abs=%v", err)
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	log.Printf("UPLOAD: destDir=%s", destDir)
	if !strings.HasPrefix(destDir, absMP) {
		http.Error(w, "path outside mount point", http.StatusForbidden)
		return
	}

	os.MkdirAll(destDir, 0755)
	log.Printf("UPLOAD: mkdirall %s", destDir)

	uniqueName := uuid.New().String()[:8] + "-" + filename
	destPath := filepath.Join(destDir, uniqueName)

	relDir, _ := filepath.Rel(absMP, destDir)
	if relDir == "." || relDir == "" {
		relDir = ""
	}
	metaData, _ := json.Marshal(map[string]string{
		"original_name": filename,
		"dir":           relDir,
	})
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



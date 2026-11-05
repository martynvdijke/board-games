package web

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"boardgames/internal/auth"
	"boardgames/internal/collection"
)

const (
	maxSetupUploadBytes = 5 << 20
	maxSetupRequestBody = 6 << 20
)

var setupImageExt = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

func (h *Handler) updateSetup(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.errorPage(w, r, http.StatusNotFound, "Game not found", nil)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.errorPage(w, r, http.StatusBadRequest, "That form couldn't be read", err)
		return
	}
	notes := r.PostForm.Get("setup_notes")
	patch := collection.CollectionPatch{SetupNotes: &notes}
	if err := h.Games.Apply(ctx, user.ID, id, patch); err != nil {
		if errors.Is(err, collection.ErrNotFound) {
			h.errorPage(w, r, http.StatusNotFound, "That game isn't on this shelf", nil)
			return
		}
		h.flashRedirect(w, r, "/games/"+strconv.FormatInt(id, 10)+"#setup", err.Error(), "error")
		return
	}
	h.flashRedirect(w, r, "/games/"+strconv.FormatInt(id, 10)+"#setup", "Setup guide saved", "success")
}

func (h *Handler) uploadSetupImage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 20*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.errorPage(w, r, http.StatusNotFound, "Game not found", nil)
		return
	}
	redirect := "/games/" + strconv.FormatInt(id, 10) + "#setup"
	fail := func(message string) {
		h.flashRedirect(w, r, redirect, message, "error")
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxSetupRequestBody)
	if err := r.ParseMultipartForm(maxSetupRequestBody); err != nil {
		fail("That upload was too large or couldn't be read")
		return
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		fail("Choose a photo to upload")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxSetupUploadBytes+1))
	if err != nil {
		fail("That photo couldn't be read")
		return
	}
	if len(data) == 0 {
		fail("That photo is empty")
		return
	}
	if len(data) > maxSetupUploadBytes {
		fail("Photos must be 5 MB or smaller")
		return
	}

	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	contentType := http.DetectContentType(head)
	ext, ok := setupImageExt[contentType]
	if !ok {
		fail("Photos must be JPEG, PNG, WebP, or GIF")
		return
	}

	caption := strings.TrimSpace(r.FormValue("caption"))
	if utf8.RuneCountInString(caption) > 200 {
		fail("Caption must be 200 characters or fewer")
		return
	}

	count, err := h.Games.CountSetupImages(ctx, user.ID, id)
	if err != nil {
		fail("That photo couldn't be saved")
		return
	}
	if count >= collection.MaxSetupImages {
		fail("A setup guide can hold up to 12 photos")
		return
	}

	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		fail("That photo couldn't be saved")
		return
	}
	imageID := hex.EncodeToString(raw[:])
	filename := imageID + ext

	if err := os.MkdirAll(h.SetupMediaDir, 0o750); err != nil {
		h.errorPage(w, r, http.StatusInternalServerError, "That photo couldn't be saved", err)
		return
	}
	finalPath := filepath.Join(h.SetupMediaDir, filename)
	tmpPath := finalPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o640); err != nil {
		h.errorPage(w, r, http.StatusInternalServerError, "That photo couldn't be saved", err)
		return
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		h.errorPage(w, r, http.StatusInternalServerError, "That photo couldn't be saved", err)
		return
	}
	if err := h.Games.AddSetupImage(ctx, user.ID, id, imageID, filename, contentType, caption); err != nil {
		_ = os.Remove(finalPath)
		if errors.Is(err, collection.ErrNotFound) {
			h.errorPage(w, r, http.StatusNotFound, "That game isn't on this shelf", nil)
			return
		}
		h.errorPage(w, r, http.StatusInternalServerError, "That photo couldn't be saved", err)
		return
	}
	h.flashRedirect(w, r, redirect, "Setup photo added", "success")
}

func (h *Handler) deleteSetupImage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.errorPage(w, r, http.StatusNotFound, "Game not found", nil)
		return
	}
	redirect := "/games/" + strconv.FormatInt(id, 10) + "#setup"
	filename, err := h.Games.DeleteSetupImage(ctx, user.ID, r.PathValue("imageID"))
	if err != nil {
		if errors.Is(err, collection.ErrNotFound) {
			h.errorPage(w, r, http.StatusNotFound, "That photo isn't here", nil)
			return
		}
		h.flashRedirect(w, r, redirect, "That photo couldn't be removed", "error")
		return
	}
	if err := os.Remove(filepath.Join(h.SetupMediaDir, filepath.Base(filename))); err != nil && !errors.Is(err, os.ErrNotExist) {
		h.flashRedirect(w, r, redirect, "The photo was removed but its file couldn't be deleted", "error")
		return
	}
	h.flashRedirect(w, r, redirect, "Setup photo removed", "success")
}

func (h *Handler) serveSetupImage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	img, err := h.Games.SetupImage(ctx, user.ID, r.PathValue("imageID"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(h.SetupMediaDir, filepath.Base(img.Filename))
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", img.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeContent(w, r, img.Filename, info.ModTime(), f)
}

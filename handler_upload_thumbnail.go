package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Thumbnail file missing", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error parsing header", err)
		return
	}
	if mediaType == "" {
		respondWithError(w, http.StatusBadRequest, "Missing Content-Type header", errors.New("Missing Content-Type header"))
		return
	}

	fileExtension := ""
	switch mediaType {
	case "image/png":
		fileExtension = ".png"
	case "image/jpeg":
		fileExtension = ".jpeg"
	default:
		respondWithError(w, http.StatusBadRequest, "Unsupported image type", errors.New("Unsupported image type"))
		return
	}

	randData := make([]byte, 32)
	_, err = rand.Read(randData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error generating file name", err)
		return
	}
	randFileName := base64.RawURLEncoding.EncodeToString(randData)

	fileName := fmt.Sprintf("%s%s", randFileName, fileExtension)
	imgPath := filepath.Join(cfg.assetsRoot, fileName)
	imgFile, err := os.Create(imgPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating new file", err)
		return
	}
	defer imgFile.Close()

	_, err = io.Copy(imgFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to write image file", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Video record does not exist", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User not authorized to edit video", errors.New("User not authorized to edit video"))
		return
	}

	thumbURL := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, fileName)
	video.ThumbnailURL = &thumbURL
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error saving to database", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

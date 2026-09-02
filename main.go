package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fiatjaf/eventstore/badger"
	"github.com/fiatjaf/eventstore/lmdb"
	"github.com/fiatjaf/eventstore/postgresql"
	"github.com/fiatjaf/khatru"
	"github.com/fiatjaf/khatru/blossom"
	"github.com/joho/godotenv"
	"github.com/nbd-wtf/go-nostr"
	"github.com/spf13/afero"
)

type Config struct {
	RelayName             string
	RelayPubkey           string
	RelayDescription      string
	DBEngine              *string
	DBPath                *string
	PostgresUser          *string
	PostgresPassword      *string
	PostgresDB            *string
	PostgresHost          *string
	PostgresPort          *string
	DatabaseURL           *string
	TeamDomain            string
	NPUBDomain            string
	BlossomEnabled        bool
	BlossomPath           *string
	BlossomURL            *string
	WebSocketURL          *string
	AllowedKinds          []int
	PublicAllowedKinds    []int
	TrustedClientName     string
	TrustedClientKinds    []int
	TrustedClientAllKinds bool
	MaxUploadSizeMB       int
	RelayPort             string
	AllowedMirrorHosts    []string
	// S3 Storage Configuration
	StorageBackend string
	S3Endpoint     string
	S3Bucket       string
	S3Region       string
	S3PublicURL    string
}

// resolveDashboardAdminPubkey picks the operator key for dashboard auth.
// Priority: in-memory data("_"), local public/.well-known/nostr.json("_"), then RELAY_PUBKEY fallback.
func resolveDashboardAdminPubkey(config Config) string {
	if pk, ok := data.Names["_"]; ok && pk != "" {
		return pk
	}

	body, err := os.ReadFile("./public/.well-known/nostr.json")
	if err == nil {
		var localData NostrData
		if err := json.Unmarshal(body, &localData); err == nil {
			if pk, ok := localData.Names["_"]; ok && pk != "" {
				return pk
			}
		}
	}

	return config.RelayPubkey
}

func truncatePubkey(pk string) string {
	if len(pk) <= 8 {
		return pk
	}
	return pk[:8]
}

type NostrData struct {
	Names  map[string]string   `json:"names"`
	Relays map[string][]string `json:"relays"`
}

var data NostrData
var relay *khatru.Relay
var db DBBackend
var fs afero.Fs
var config Config
var s3Storage *S3Storage

func main() {
	relay = khatru.NewRelay()
	config := LoadConfig()

	// Initialize nostr.json with relay pubkey as root if needed
	if err := initializeNostrJson(config); err != nil {
		log.Printf("Warning: Failed to initialize nostr.json: %s", err)
	}

	relay.StoreEvent = append(relay.StoreEvent, db.SaveEvent)
	relay.QueryEvents = append(relay.QueryEvents, db.QueryEvents)
	relay.DeleteEvent = append(relay.DeleteEvent, db.DeleteEvent)

	fetchNostrData(config.NPUBDomain)

	// Apply spam protection policies
	applySpamProtection(relay, config)

	go func() {
		for {
			time.Sleep(1 * time.Hour)
			fetchNostrData(config.NPUBDomain)
		}
	}()

	relay.RejectEvent = append(relay.RejectEvent, func(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
		// Check for trusted client exception: allow specific kinds (or all kinds) from a specific client
		trustedClientException := false
		if config.TrustedClientName != "" {
			for _, tag := range event.Tags {
				if len(tag) >= 2 && tag[0] == "client" && tag[1] == config.TrustedClientName {
					// If all kinds allowed for trusted client, allow immediately
					if config.TrustedClientAllKinds {
						trustedClientException = true
						break
					}
					// Otherwise check specific kinds
					for _, kc := range config.TrustedClientKinds {
						if event.Kind == kc {
							trustedClientException = true
							break
						}
					}
					if trustedClientException {
						break
					}
				}
			}
		}
		if trustedClientException {
			return false, "" // allow event from trusted client for configured kinds
		}

		// Check if this is a delete event (kind 5)
		if event.Kind == 5 {
			// Team members can delete any events
			for _, pubkey := range data.Names {
				if event.PubKey == pubkey {
					return false, "" // allow team members to delete any events
				}
			}

			// Public users can delete their own posts if they have "e" tags referencing events
			// and the original event was posted via PUBLIC_ALLOWED_KINDS
			if len(config.PublicAllowedKinds) > 0 {
				// Check if the delete event has "e" tags (references to events being deleted)
				hasEventRefs := false
				for _, tag := range event.Tags {
					if len(tag) >= 2 && tag[0] == "e" {
						hasEventRefs = true
						break
					}
				}

				if hasEventRefs {
					// Allow public users to delete (they can only delete their own events
					// as the relay will verify ownership when processing the delete)
					return false, "" // allow public users to delete their own events
				}
			}

			return true, "only team members can delete events, or users can delete their own posts"
		}

		// Check if this is a public allowed kind (any pubkey can post these)
		if len(config.PublicAllowedKinds) > 0 {
			for _, publicKind := range config.PublicAllowedKinds {
				if event.Kind == publicKind {
					return false, "" // allow public posting for this kind
				}
			}
		}

		// Check if user is part of the team
		isTeamMember := false
		for _, pubkey := range data.Names {
			if event.PubKey == pubkey {
				isTeamMember = true
				break
			}
		}
		if !isTeamMember {
			return true, "you are not part of the team"
		}

		// Check if event kind is allowed for team members
		if len(config.AllowedKinds) > 0 {
			isKindAllowed := false
			for _, allowedKind := range config.AllowedKinds {
				if event.Kind == allowedKind {
					isKindAllowed = true
					break
				}
			}
			if !isKindAllowed {
				return true, fmt.Sprintf("event kind %d is not allowed for team members", event.Kind)
			}
		}

		return false, "" // allow
	})

	// Setup front page handler
	setupFrontPageHandler(relay, config)

	// Setup dashboard handlers
	setupDashboardHandlers(relay, config)

	// Start background zap stats computation (community aggregate analytics)
	go startZapStatsBackground(config)

	// Add handler for all public assets
	relay.Router().HandleFunc("/public/", func(w http.ResponseWriter, r *http.Request) {
		// Get the requested file path (remove /public/ prefix)
		requestedPath := strings.TrimPrefix(r.URL.Path, "/public/")

		// Prevent directory traversal attacks
		if strings.Contains(requestedPath, "..") {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		// Serve the file from public directory
		filePath := "./public/" + requestedPath
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}

		http.ServeFile(w, r, filePath)
	})

	setupConvertHandlers(relay, config)

	// Setup Scheduler - use configured DB path
	schedulerDataPath := *config.DBPath
	scheduler, err := NewScheduler(schedulerDataPath)
	if err != nil {
		log.Printf("Failed to initialize scheduler: %v", err)
	} else {
		scheduler.Start()
		relay.Router().HandleFunc("/api/scheduler/schedule", scheduler.HandleSchedule)
		relay.Router().HandleFunc("/api/scheduler/list", scheduler.HandleList)
		relay.Router().HandleFunc("/api/scheduler/delete", scheduler.HandleDelete)
	}

	// Health check endpoint for scheduler API
	relay.Router().HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Add NIP-05 service handlers
	//	setupNIP05Handlers(relay, config)

	if !config.BlossomEnabled {
		// Configure HTTP server with timeouts suitable for large file uploads
		server := &http.Server{
			Addr:              ":" + config.RelayPort,
			Handler:           corsAllowCredentials(relay),
			ReadTimeout:       15 * time.Minute, // Increased to 15 minutes for very large files
			WriteTimeout:      15 * time.Minute, // Increased to 15 minutes
			IdleTimeout:       5 * time.Minute,  // Increased idle timeout
			ReadHeaderTimeout: 30 * time.Second, // Prevent slow header attacks
			MaxHeaderBytes:    1 << 20,          // 1MB max header size
		}

		fmt.Println("running on :" + config.RelayPort + " with extended timeouts for large uploads")
		server.ListenAndServe()
		return
	}

	bl := blossom.New(relay, *config.BlossomURL)
	bl.Store = blossom.EventStoreBlobIndexWrapper{Store: db, ServiceURL: bl.ServiceURL}

	if config.StorageBackend == "s3" && s3Storage != nil {
		// S3 Storage Backend
		bl.StoreBlob = append(bl.StoreBlob, func(ctx context.Context, sha256 string, body []byte) error {
			return s3Storage.StoreBlob(ctx, sha256, body)
		})

		bl.LoadBlob = append(bl.LoadBlob, func(ctx context.Context, sha256 string) (io.ReadSeeker, error) {
			reader, redirectURL, err := s3Storage.LoadBlob(ctx, sha256)
			if err != nil {
				return nil, err
			}
			// If we have a redirect URL, we need to handle it differently
			// The khatru blossom library expects just ReadSeeker, so we return the reader
			// For S3 with public URL, the redirect is handled via the public URL config
			if redirectURL != nil {
				log.Printf("LoadBlob: S3 redirect URL available: %s", redirectURL.String())
			}
			return reader, nil
		})

		bl.DeleteBlob = append(bl.DeleteBlob, func(ctx context.Context, sha256 string) error {
			return s3Storage.DeleteBlob(ctx, sha256)
		})
	} else {
		// Filesystem Storage Backend
		bl.StoreBlob = append(bl.StoreBlob, func(ctx context.Context, sha256 string, body []byte) error {
			// Create context with timeout for large file operations
			storeCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			defer cancel()

			file, err := fs.Create(*config.BlossomPath + sha256)
			if err != nil {
				return err
			}
			defer file.Close()

			// Use streaming copy with context checking for large files
			reader := bytes.NewReader(body)
			buffer := make([]byte, 32*1024) // 32KB buffer for efficient copying

			for {
				select {
				case <-storeCtx.Done():
					return storeCtx.Err()
				default:
				}

				n, err := reader.Read(buffer)
				if n > 0 {
					if _, writeErr := file.Write(buffer[:n]); writeErr != nil {
						return writeErr
					}
				}
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
			}

			// Detect and store MIME type in sidecar. This runs for every
			// upload, complementing the /mirror endpoint which does its
			// own MIME capture. Uses detectMimeType() which handles
			// QuickTime and other formats that Go's built-in sniffer misses.
			mimeType := detectMimeType(body)
			if mimeType != "" && mimeType != "application/octet-stream" {
				setBlobMime(*config.BlossomPath, sha256, mimeType)
			}

			return file.Sync() // Ensure data is written to disk
		})

		bl.LoadBlob = append(bl.LoadBlob, func(ctx context.Context, sha256 string) (io.ReadSeeker, error) {
			filePath := *config.BlossomPath + sha256
			log.Printf("LoadBlob: Attempting to open file at path: %s", filePath)
			file, err := fs.Open(filePath)
			if err != nil {
				log.Printf("LoadBlob: Failed to open file %s: %v", filePath, err)
				return nil, err
			}
			log.Printf("LoadBlob: Successfully opened file %s", filePath)
			return file, nil
		})

		bl.DeleteBlob = append(bl.DeleteBlob, func(ctx context.Context, sha256 string) error {
			return fs.Remove(*config.BlossomPath + sha256)
		})
	}
	bl.RejectUpload = append(bl.RejectUpload, func(ctx context.Context, event *nostr.Event, size int, ext string) (bool, string, int) {
		// Check for configurable size limit
		maxSize := config.MaxUploadSizeMB * 1024 * 1024
		if size > maxSize {
			return true, fmt.Sprintf("file size exceeds %dMB limit", config.MaxUploadSizeMB), 413
		}

		for _, pubkey := range data.Names {
			if pubkey == event.PubKey {
				return false, ext, size
			}
		}

		return true, "you are not part of the team", 403
	})

	// Restrict blob deletion to publishers and the master user only.
	// "user" role team members can upload and view media but cannot delete.
	// The adminRoles map is read from the kind 30078 site-config event
	// stored in the relay DB (same source as the frontend).
	bl.RejectDelete = append(bl.RejectDelete, func(ctx context.Context, auth *nostr.Event, sha256 string) (bool, string, int) {
		if auth == nil {
			return true, "missing authorization", 401
		}
		pk := strings.ToLower(strings.TrimSpace(auth.PubKey))

		// The master user (the "_" entry in nostr.json) can always delete
		masterPubkey := ""
		if pk2, ok := data.Names["_"]; ok && pk2 != "" {
			masterPubkey = strings.ToLower(pk2)
		}
		if pk == masterPubkey {
			return false, "", 0
		}

		// Look up adminRoles from the kind 30078 site-config event.
		adminRoles := getAdminRoles(ctx, db, masterPubkey)
		if role, ok := adminRoles[pk]; ok && role == "publisher" {
			return false, "", 0
		}

		return true, "only publishers can delete media", 403
	})

	// Add custom list endpoint for Sakura health checks
	relay.Router().HandleFunc("/list/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract pubkey from URL path
		pubkey := strings.TrimPrefix(r.URL.Path, "/list/")
		if pubkey == "" {
			http.Error(w, "Missing pubkey", http.StatusBadRequest)
			return
		}

		log.Printf("List blobs request for pubkey: %s", pubkey)

		// Build a map of sha256 → created_at from kind 24242 events.
		// These events store an upload timestamp, but it may be inaccurate
		// (it reflects when the 24242 event was created, which could be
		// harvest time, not original upload time). We use it only as a
		// fallback when the file modification time is missing or newer
		// (which would indicate the file was recently re-written without
		// an X-Original-Date header).
		uploadTimes := make(map[string]int64)
		ech, err := db.QueryEvents(r.Context(), nostr.Filter{Kinds: []int{24242}, Limit: 0})
		if err == nil {
			for evt := range ech {
				for _, tag := range evt.Tags {
					if len(tag) >= 2 && tag[0] == "x" {
						sha := strings.ToLower(tag[1])
						t := evt.CreatedAt.Time().Unix()
						// Keep the earliest upload time if there are duplicates
						if existing, ok := uploadTimes[sha]; !ok || t < existing {
							uploadTimes[sha] = t
						}
					}
				}
			}
		}

		// Read all files from storage backend
		blobs := []map[string]interface{}{}

		// Load the owner map (sha256 → pubkey) from the sidecar file
		var ownerMap map[string]string
		var mimeMap map[string]string
		if config.BlossomPath != nil {
			ownerMap = readOwnerMap(*config.BlossomPath)
			mimeMap = readMimeMap(*config.BlossomPath)
		}

		if config.StorageBackend == "s3" && s3Storage != nil {
			// S3 Storage Backend
			s3Blobs, err := s3Storage.ListBlobs(r.Context())
			if err != nil {
				log.Printf("Error listing S3 blobs: %v", err)
			} else {
				for _, blob := range s3Blobs {
					uploaded := blob.Uploaded
					if t24242, ok := uploadTimes[strings.ToLower(blob.SHA256)]; ok {
						if uploaded == 0 || uploaded > t24242 {
							uploaded = t24242
						}
					}
					blobs = append(blobs, map[string]interface{}{
						"sha256":   blob.SHA256,
						"size":     blob.Size,
						"type":     blob.Type,
						"url":      blob.URL,
						"uploaded": uploaded,
						"owner":    ownerMap[strings.ToLower(blob.SHA256)],
					})
				}
			}
		} else if config.BlossomPath != nil {
			// Filesystem Storage Backend
			file, err := fs.Open(*config.BlossomPath)
			if err != nil {
				log.Printf("Error opening blossom directory: %v", err)
			} else {
				defer file.Close()
				fileInfos, err := file.Readdir(-1)
				if err != nil {
					log.Printf("Error reading blossom directory: %v", err)
				} else {
					for _, fileInfo := range fileInfos {
						if !fileInfo.IsDir() {
							fileName := fileInfo.Name()
							// Validate that it looks like a SHA256 hash (64 hex characters)
							if len(fileName) == 64 {
								isValidHash := true
								for _, char := range fileName {
									if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
										isValidHash = false
										break
									}
								}

								if isValidHash {
									sha := strings.ToLower(fileName)
									// Determine MIME type: prefer the sidecar map
									// (captured at upload or mirror time). Fall
									// back to detectMimeType() which handles
									// QuickTime and other ISO BMFF formats that
									// Go's built-in sniffer misses.
									contentType := "application/octet-stream" // Default fallback
									if storedMime, ok := mimeMap[sha]; ok && storedMime != "" {
										contentType = storedMime
									} else {
										filePath := *config.BlossomPath + fileName
										if blobFile, err := fs.Open(filePath); err == nil {
											buffer := make([]byte, 512)
											if n, err := blobFile.Read(buffer); err == nil && n > 0 {
												detectedType := detectMimeType(buffer[:n])
												if detectedType != "" {
													contentType = detectedType
												}
											}
											blobFile.Close()
										}
									}

									// Skip non-media blobs (e.g. HTML pages that were
									// accidentally mirrored from CDN error pages)
									if strings.HasPrefix(contentType, "text/html") {
										continue
									}

									// Prefer the file modification time (set by the
									// harvest via os.Chtimes with X-Original-Date).
									// Fall back to 24242 event timestamp only if the
									// file mod time is missing or appears wrong
									// (newer than the 24242 timestamp, meaning the
									// file was written without X-Original-Date).
									fileTime := fileInfo.ModTime().Unix()
									uploaded := fileTime
									if t24242, ok := uploadTimes[sha]; ok {
										if fileTime == 0 || fileTime > t24242 {
											uploaded = t24242
										}
									}

									blob := map[string]interface{}{
										"sha256":   sha,
										"size":     fileInfo.Size(),
										"type":     contentType,
										"url":      *config.BlossomURL + "/" + sha,
										"uploaded": uploaded,
										"owner":    ownerMap[sha],
									}
									blobs = append(blobs, blob)
								}
							}
						}
					}
				}
			}
		}

		// Sort by uploaded date descending (newest first)
		sort.Slice(blobs, func(i, j int) bool {
			iv, _ := blobs[i]["uploaded"].(int64)
			jv, _ := blobs[j]["uploaded"].(int64)
			return iv > jv
		})

		log.Printf("Returning %d blobs for pubkey %s", len(blobs), pubkey)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(blobs)
	})

	// Backfill the blob owner map by scanning all events for imeta/url tags.
	// This is a one-time operation to populate owners for blobs that were
	// harvested before the X-Owner-Pubkey header was added. After this runs,
	// the harvest automatically keeps the map up to date.
	relay.Router().HandleFunc("/backfill-owners", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if config.BlossomPath == nil {
			http.Error(w, "Blossom path not configured", http.StatusInternalServerError)
			return
		}

		// Load the existing owner map so we don't lose any entries
		ownerMap := readOwnerMap(*config.BlossomPath)
		beforeCount := len(ownerMap)

		// Scan all events for media URLs and map sha256 → pubkey
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()

		// Query all events (no kind filter — media can appear in any event kind)
		ech, err := db.QueryEvents(ctx, nostr.Filter{Limit: 0})
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to query events: %v", err), http.StatusInternalServerError)
			return
		}

		sha256Regex := regexp.MustCompile(`\b[0-9a-f]{64}\b`)

		for evt := range ech {
			pubkey := strings.ToLower(evt.PubKey)
			if pubkey == "" {
				continue
			}

			// Check imeta tags: ['imeta', 'url https://...', 'm image/jpeg', ...]
			for _, tag := range evt.Tags {
				if tag[0] == "imeta" {
					for _, part := range tag[1:] {
						if strings.HasPrefix(part, "url ") {
							url := strings.TrimSpace(part[4:])
							// Extract sha256 from the URL
							if m := sha256Regex.FindString(url); m != "" {
								sha := strings.ToLower(m)
								// Keep the first owner we find (earliest event wins)
								if _, exists := ownerMap[sha]; !exists {
									ownerMap[sha] = pubkey
								}
							}
						}
					}
				}
				// Check image/thumb/banner/picture tags
				if len(tag) >= 2 && (tag[0] == "image" || tag[0] == "thumb" || tag[0] == "banner" || tag[0] == "picture") {
					if m := sha256Regex.FindString(tag[1]); m != "" {
						sha := strings.ToLower(m)
						if _, exists := ownerMap[sha]; !exists {
							ownerMap[sha] = pubkey
						}
					}
				}
			}

			// Check event content for URLs with sha256 hashes
			if evt.Content != "" {
				for _, m := range sha256Regex.FindAllString(evt.Content, -1) {
					sha := strings.ToLower(m)
					if _, exists := ownerMap[sha]; !exists {
						ownerMap[sha] = pubkey
					}
				}
			}

			// For kind 0 (profile), check JSON content for picture/banner
			if evt.Kind == 0 {
				var profile map[string]interface{}
				if err := json.Unmarshal([]byte(evt.Content), &profile); err == nil {
					for _, field := range []string{"picture", "banner", "image"} {
						if val, ok := profile[field].(string); ok {
							if m := sha256Regex.FindString(val); m != "" {
								sha := strings.ToLower(m)
								if _, exists := ownerMap[sha]; !exists {
									ownerMap[sha] = pubkey
								}
							}
						}
					}
				}
			}
		}

		// Also check 24242 events for owner info (the event pubkey is the owner)
		ech24242, err := db.QueryEvents(ctx, nostr.Filter{Kinds: []int{24242}, Limit: 0})
		if err == nil {
			for evt := range ech24242 {
				pubkey := strings.ToLower(evt.PubKey)
				for _, tag := range evt.Tags {
					if len(tag) >= 2 && tag[0] == "x" {
						sha := strings.ToLower(tag[1])
						if _, exists := ownerMap[sha]; !exists {
							ownerMap[sha] = pubkey
						}
					}
				}
			}
		}

		// Write the updated owner map
		metaDir := *config.BlossomPath + ".metadata"
		_ = os.MkdirAll(metaDir, 0755)
		data, err := json.Marshal(ownerMap)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to marshal owner map: %v", err), http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(getOwnerMapPath(*config.BlossomPath), data, 0644); err != nil {
			http.Error(w, fmt.Sprintf("Failed to write owner map: %v", err), http.StatusInternalServerError)
			return
		}

		added := len(ownerMap) - beforeCount
		log.Printf("Backfill complete: %d total owners (%d new), was %d before", len(ownerMap), added, beforeCount)

		// Count how many of our actual blobs now have owners
		blobCount := 0
		mappedCount := 0
		if files, err := os.ReadDir(*config.BlossomPath); err == nil {
			for _, f := range files {
				if !f.IsDir() && len(f.Name()) == 64 {
					blobCount++
					if _, ok := ownerMap[strings.ToLower(f.Name())]; ok {
						mappedCount++
					}
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_owners_in_map": len(ownerMap),
			"new_entries_added":   added,
			"total_blobs":         blobCount,
			"blobs_with_owner":    mappedCount,
			"coverage_pct":        func() int {
				if blobCount == 0 {
					return 0
				}
				return mappedCount * 100 / blobCount
			}(),
		})
	})

	// Add custom mirror endpoint handler for Sakura compatibility
	relay.Router().HandleFunc("/mirror", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the request body to get source URL
		var mirrorRequest struct {
			URL string `json:"url"`
		}

		if err := json.NewDecoder(r.Body).Decode(&mirrorRequest); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if mirrorRequest.URL == "" {
			http.Error(w, "Missing source URL", http.StatusBadRequest)
			return
		}

		// Optional: the harvest frontend sends the original event's
		// created_at as a header so we can set the file's modification
		// time to the original publication date. This makes the /list/
		// endpoint sort blobs by when the media was originally posted,
		// not when it was harvested/mirrored.
		var originalDate time.Time
		if dateStr := r.Header.Get("X-Original-Date"); dateStr != "" {
			if ts, err := strconv.ParseInt(dateStr, 10, 64); err == nil {
				originalDate = time.Unix(ts, 0)
			}
		}

		// Optional: the harvest frontend sends the owner's pubkey so we
		// can record who owns each blob in the sidecar owner map. This
		// enables filtering media by user in the media browser.
		var ownerPubkey string
		if pk := r.Header.Get("X-Owner-Pubkey"); pk != "" {
			ownerPubkey = strings.ToLower(strings.TrimSpace(pk))
		}

		// Validate URL against allowlist to prevent SSRF attacks
		if !isAllowedMirrorURL(mirrorRequest.URL, config.AllowedMirrorHosts) {
			http.Error(w, "Source URL host not in allowed list", http.StatusForbidden)
			return
		}

		// Store validated URL to make it clear to static analysis that it's safe
		validatedURL := mirrorRequest.URL

		// Extract blob hash from source URL if present (optional — we compute it from content regardless)
		expectedHash := extractSha256FromURL(validatedURL)

		// If we already have the blob, return immediately
		if expectedHash != "" {
			if _, err := fs.Open(*config.BlossomPath + expectedHash); err == nil {
				// Even if the blob exists, update its file mod time to the
				// original date if provided. This fixes blobs that were
				// previously harvested without a date (their mod time was
				// set to harvest time).
				if !originalDate.IsZero() {
					_ = os.Chtimes(*config.BlossomPath+expectedHash, originalDate, originalDate)
				}
				// Record the owner in the sidecar map
				if ownerPubkey != "" {
					setBlobOwner(*config.BlossomPath, expectedHash, ownerPubkey)
				}
				response := map[string]interface{}{
					"sha256": expectedHash,
					"url":    *config.BlossomURL + "/" + expectedHash,
					"size":   0,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}
		}

		// Download blob from validated source URL
		resp, err := http.Get(validatedURL)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to fetch source blob: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			http.Error(w, fmt.Sprintf("Source server returned %d", resp.StatusCode), http.StatusBadGateway)
			return
		}

		// Read the blob content
		blobData, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read blob data: %v", err), http.StatusInternalServerError)
			return
		}

		// Capture the MIME type from the source response. Fall back to
		// content sniffing if the source didn't send a Content-Type.
		// This is stored in a sidecar file so the GET handler can set the
		// correct Content-Type header when serving the blob (mirrored blobs
		// have no kind 24242 blob descriptor, so http.ServeContent can't
		// infer the type from a blob descriptor or URL extension).
		sourceMime := resp.Header.Get("Content-Type")
		if sourceMime == "" {
			sourceMime = http.DetectContentType(blobData)
		}

		// Compute actual hash from content
		hasher := sha256.New()
		hasher.Write(blobData)
		actualHash := hex.EncodeToString(hasher.Sum(nil))

		// Note: we intentionally do NOT reject hash mismatches.
		// Some CDNs (e.g. image.nostr.build) resize/transcode images so the
		// downloaded bytes differ from the original. We store whatever we get
		// under its actual content hash — the URL hash is just a hint for
		// early-exit deduplication, not a correctness gate.

		// If blob was already stored under the computed hash, return immediately
		if expectedHash == "" {
			if _, err := fs.Open(*config.BlossomPath + actualHash); err == nil {
				if !originalDate.IsZero() {
					_ = os.Chtimes(*config.BlossomPath+actualHash, originalDate, originalDate)
				}
				if ownerPubkey != "" {
					setBlobOwner(*config.BlossomPath, actualHash, ownerPubkey)
				}
				setBlobMime(*config.BlossomPath, actualHash, sourceMime)
				response := map[string]interface{}{
					"sha256": actualHash,
					"url":    *config.BlossomURL + "/" + actualHash,
					"size":   0,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}
		}

		blobHash := actualHash

		// Store the blob using the existing StoreBlob functionality
		ctx := r.Context()
		for _, storeFunc := range bl.StoreBlob {
			if err := storeFunc(ctx, blobHash, blobData); err != nil {
				http.Error(w, fmt.Sprintf("Failed to store blob: %v", err), http.StatusInternalServerError)
				return
			}
		}

		// Note: we intentionally do NOT call bl.Store.Keep() here.
		// Keep() creates a kind-24242 event but doesn't sign it (the
		// Blossom library has no access to the user's private key).
		// Unsigned events are silently dropped by Nostr clients (like
		// nostrify's NRelay1) which verify signatures, causing them to
		// not count toward the query limit and breaking pagination.
		// Instead, the /list/ endpoint uses file modification time as
		// the upload timestamp for mirrored blobs.

		// Set the file's modification time to the original event's
		// created_at so the /list/ endpoint sorts by original publication
		// date, not by harvest/mirror time.
		if !originalDate.IsZero() {
			_ = os.Chtimes(*config.BlossomPath+blobHash, originalDate, originalDate)
		}

		// Record the owner in the sidecar map
		if ownerPubkey != "" {
			setBlobOwner(*config.BlossomPath, blobHash, ownerPubkey)
		}

		// Record the MIME type in the sidecar map
		setBlobMime(*config.BlossomPath, blobHash, sourceMime)

		// Return success response
		response := map[string]interface{}{
			"sha256": blobHash,
			"url":    *config.BlossomURL + "/" + blobHash,
			"size":   len(blobData),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

		log.Printf("Successfully mirrored blob %s from %s", blobHash, validatedURL)
	})

	// --- Video processing endpoint ---
	// Transcodes a stored video blob to MP4 (H.264/AAC) using ffmpeg,
	// stripping all metadata and optimizing for streaming. The original
	// blob is preserved until the client confirms acceptance (via DELETE).
	// Runs at lowest CPU priority (nice -n 19) with a single-job semaphore
	// so the relay is never starved on this 2-core VM.
	videoProcessSem := make(chan struct{}, 1) // 1 concurrent job

	// validateVideoQuality maps a quality preset string to a CRF value.
	// Returns -1 if the quality string is invalid.
	validateVideoQuality := func(quality string) (int, bool) {
		switch quality {
		case "high":
			return 18, true
		case "medium":
			return 23, true
		case "low":
			return 28, true
		case "none":
			return 0, true // no re-encoding, just remux + strip metadata
		default:
			return 0, false
		}
	}

	// validateVideoResolution maps a resolution string to a max height.
	// Returns -1 if the resolution string is invalid.
	validateVideoResolution := func(resolution string) (int, bool) {
		switch resolution {
		case "original", "":
			return 0, true
		case "1080":
			return 1080, true
		case "720":
			return 720, true
		case "480":
			return 480, true
		default:
			return 0, false
		}
	}

	// buildFFmpegArgs constructs the ffmpeg command args for transcoding
	// to MP4 (H.264/AAC) with metadata stripped and +faststart for streaming.
	buildFFmpegArgs := func(tmpInput, tmpOutput string, crf, maxHeight int) []string {
		args := []string{"-n", "19", "ffmpeg", "-i", tmpInput}
		if maxHeight > 0 {
			args = append(args, "-vf", fmt.Sprintf("scale=-2:%d", maxHeight))
		}
		args = append(args,
			"-c:v", "libx264",
			"-crf", strconv.Itoa(crf),
			"-preset", "medium",
			"-c:a", "aac",
			"-b:a", "128k",
			"-map_metadata", "-1",
			"-movflags", "+faststart",
			"-y",
			tmpOutput,
		)
		return args
	}

	// buildFFmpegCopyArgs constructs the ffmpeg command args for a copy-only
	// remux (no re-encoding). This strips container-level metadata and adds
	// +faststart for streaming, but preserves the original video/audio codecs.
	// Near-instant compared to a full transcode. Used when quality="none".
	buildFFmpegCopyArgs := func(tmpInput, tmpOutput string) []string {
		return []string{
			"-n", "19", "ffmpeg", "-i", tmpInput,
			"-c", "copy",
			"-map_metadata", "-1",
			"-movflags", "+faststart",
			"-y",
			tmpOutput,
		}
	}

	// runFFmpeg executes ffmpeg at lowest CPU priority and logs failures.
	// logPrefix is included in the failure log message for identification.
	runFFmpeg := func(ctx context.Context, args []string, logPrefix string) error {
		cmd := exec.CommandContext(ctx, "nice", args...)
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			if logPrefix != "" {
				log.Printf("ffmpeg failed for %s: %v", logPrefix, err)
			} else {
				log.Printf("ffmpeg failed: %v", err)
			}
			return err
		}
		return nil
	}

	// storeProcessedBlob reads the transcoded file, computes its SHA-256,
	// stores it in the blossom path, and records MIME type + owner.
	// Returns (hash, processedSize, error).
	storeProcessedBlob := func(tmpOutput string, blossomPath string, ownerPubkey string) (string, int, error) {
		processedData, err := os.ReadFile(tmpOutput)
		if err != nil {
			return "", 0, err
		}
		hasher := sha256.New()
		hasher.Write(processedData)
		processedHash := hex.EncodeToString(hasher.Sum(nil))

		processedPath := blossomPath + processedHash
		if err := os.WriteFile(processedPath, processedData, 0644); err != nil {
			return "", 0, err
		}
		setBlobMime(blossomPath, processedHash, "video/mp4")
		setBlobOwner(blossomPath, processedHash, ownerPubkey)
		return processedHash, len(processedData), nil
	}

	// acquireVideoSem tries to acquire the processing semaphore. Returns
	// false and writes a 503 JSON response if another job is running.
	acquireVideoSem := func(w http.ResponseWriter) bool {
		select {
		case videoProcessSem <- struct{}{}:
			return true
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "another video is being processed, please try again in a moment",
				"retry": true,
			})
			return false
		}
	}

	relay.Router().HandleFunc("/process-video", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Require Blossom auth (same as upload)
		auth, err := readBlossomAuth(r)
		if err != nil || auth == nil {
			http.Error(w, "Authorization required", http.StatusUnauthorized)
			return
		}

		var req struct {
			Sha256     string `json:"sha256"`
			Quality    string `json:"quality"`    // "high" | "medium" | "low"
			Resolution string `json:"resolution"` // "original" | "1080" | "720" | "480"
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}
		if req.Sha256 == "" || config.BlossomPath == nil {
			http.Error(w, "Missing sha256 or blossom not configured", http.StatusBadRequest)
			return
		}

		// Validate quality preset → CRF value
		crf, ok := validateVideoQuality(req.Quality)
		if !ok {
			http.Error(w, "Invalid quality (use: high, medium, low, none)", http.StatusBadRequest)
			return
		}

		// Validate resolution → max height (0 = original)
		maxHeight, ok := validateVideoResolution(req.Resolution)
		if !ok {
			http.Error(w, "Invalid resolution (use: original, 1080, 720, 480)", http.StatusBadRequest)
			return
		}

		// Validate hash format
		hashLower := strings.ToLower(strings.TrimSpace(req.Sha256))
		if len(hashLower) != 64 || !isHex(hashLower) {
			http.Error(w, "Invalid sha256", http.StatusBadRequest)
			return
		}

		// Check the source blob exists
		srcPath := *config.BlossomPath + hashLower
		if _, err := os.Stat(srcPath); err != nil {
			http.Error(w, "Source blob not found", http.StatusNotFound)
			return
		}

		// Acquire semaphore (non-blocking — return 503 if busy)
		if !acquireVideoSem(w) {
			return
		}
		defer func() { <-videoProcessSem }()

		// Create temp files
		tmpDir, err := os.MkdirTemp("", "ffmpeg-process-*")
		if err != nil {
			http.Error(w, "Failed to create temp dir", http.StatusInternalServerError)
			return
		}
		defer os.RemoveAll(tmpDir)

		tmpInput := filepath.Join(tmpDir, "input")
		tmpOutput := filepath.Join(tmpDir, "output.mp4")

		// Symlink the source blob (avoid copying large files)
		if err := os.Symlink(srcPath, tmpInput); err != nil {
			http.Error(w, "Failed to link source blob", http.StatusInternalServerError)
			return
		}

		// Build and run ffmpeg
		ffmpegArgs := buildFFmpegArgs(tmpInput, tmpOutput, crf, maxHeight)

		resLabel := req.Resolution
		if resLabel == "" {
			resLabel = "original"
		}
		log.Printf("Processing video %s with CRF %d, resolution %s", hashLower, crf, resLabel)
		if err := runFFmpeg(r.Context(), ffmpegArgs, hashLower); err != nil {
			http.Error(w, "Video processing failed", http.StatusInternalServerError)
			return
		}

		// Store the processed blob
		ownerPubkey := strings.ToLower(strings.TrimSpace(auth.PubKey))
		processedHash, processedSize, err := storeProcessedBlob(tmpOutput, *config.BlossomPath, ownerPubkey)
		if err != nil {
			http.Error(w, "Failed to store processed video", http.StatusInternalServerError)
			return
		}

		// Get original size for comparison
		origInfo, _ := os.Stat(srcPath)
		origSize := int64(0)
		if origInfo != nil {
			origSize = origInfo.Size()
		}

		log.Printf("Processed video %s → %s (CRF %d): %d → %d bytes (%.1f%%)",
			hashLower, processedHash, crf, origSize, processedSize,
			float64(processedSize)/float64(origSize)*100)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sha256":        processedHash,
			"url":           *config.BlossomURL + "/" + processedHash,
			"size":          processedSize,
			"original_size": origSize,
			"original_sha":  hashLower,
			"mime":          "video/mp4",
		})
	})

	// /process-video-stream — streaming transcode endpoint.
	//
	// Accepts the raw video as the request body (streamed, no arrayBuffer),
	// transcodes it with ffmpeg, and stores only the result. The original
	// is never persisted to blossom — it goes straight to a temp file,
	// through ffmpeg, and the temp file is deleted.
	//
	// Quality and resolution are passed as query params:
	//   PUT /process-video-stream?quality=medium&resolution=720
	//
	// This avoids the iOS Safari memory crash that occurs when the client
	// calls file.arrayBuffer() to compute a SHA-256 hash before uploading.
	// With this endpoint, the client streams the File directly via fetch()
	// body — no hash computation needed on the client side.
	relay.Router().HandleFunc("/process-video-stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Require Blossom auth (same as upload)
		auth, err := readBlossomAuth(r)
		if err != nil || auth == nil {
			http.Error(w, "Authorization required", http.StatusUnauthorized)
			return
		}

		if config.BlossomPath == nil {
			http.Error(w, "Blossom not configured", http.StatusBadRequest)
			return
		}

		// Parse quality from query params
		quality := r.URL.Query().Get("quality")
		if quality == "" {
			quality = "medium"
		}
		crf, ok := validateVideoQuality(quality)
		if !ok {
			http.Error(w, "Invalid quality (use: high, medium, low, none)", http.StatusBadRequest)
			return
		}

		// Parse resolution from query params
		resolution := r.URL.Query().Get("resolution")
		if resolution == "" {
			resolution = "original"
		}
		maxHeight, ok := validateVideoResolution(resolution)
		if !ok {
			http.Error(w, "Invalid resolution (use: original, 1080, 720, 480)", http.StatusBadRequest)
			return
		}

		// Acquire semaphore (non-blocking — return 503 if busy)
		if !acquireVideoSem(w) {
			return
		}
		defer func() { <-videoProcessSem }()

		// Create temp directory for input + output
		tmpDir, err := os.MkdirTemp("", "ffmpeg-stream-*")
		if err != nil {
			http.Error(w, "Failed to create temp dir", http.StatusInternalServerError)
			return
		}
		defer os.RemoveAll(tmpDir)

		tmpInput := filepath.Join(tmpDir, "input")
		tmpOutput := filepath.Join(tmpDir, "output.mp4")

		// Stream the request body to the temp input file.
		// This avoids loading the entire video into memory.
		inFile, err := os.Create(tmpInput)
		if err != nil {
			http.Error(w, "Failed to create temp input file", http.StatusInternalServerError)
			return
		}

		// Track original size as we stream
		origSize, err := io.Copy(inFile, r.Body)
		inFile.Close()
		if err != nil {
			log.Printf("Failed to stream video to temp file: %v", err)
			http.Error(w, "Failed to receive video data", http.StatusInternalServerError)
			return
		}

		if origSize == 0 {
			http.Error(w, "Empty request body", http.StatusBadRequest)
			return
		}

		// Build and run ffmpeg
		var ffmpegArgs []string
		if quality == "none" {
			ffmpegArgs = buildFFmpegCopyArgs(tmpInput, tmpOutput)
			log.Printf("Streaming copy (no transcode): %d bytes", origSize)
		} else {
			ffmpegArgs = buildFFmpegArgs(tmpInput, tmpOutput, crf, maxHeight)
			log.Printf("Streaming transcode: %d bytes, CRF %d, resolution %s", origSize, crf, resolution)
		}
		if err := runFFmpeg(r.Context(), ffmpegArgs, "streaming transcode"); err != nil {
			http.Error(w, "Video processing failed", http.StatusInternalServerError)
			return
		}

		// Store the processed blob
		ownerPubkey := strings.ToLower(strings.TrimSpace(auth.PubKey))
		processedHash, processedSize, err := storeProcessedBlob(tmpOutput, *config.BlossomPath, ownerPubkey)
		if err != nil {
			http.Error(w, "Failed to store processed video", http.StatusInternalServerError)
			return
		}

		log.Printf("Streaming transcode complete: %d → %d bytes (%.1f%%), hash %s",
			origSize, processedSize,
			float64(processedSize)/float64(origSize)*100,
			processedHash)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sha256":        processedHash,
			"url":           *config.BlossomURL + "/" + processedHash,
			"size":          processedSize,
			"original_size": origSize,
			"original_sha":  "", // no original stored
			"mime":          "video/mp4",
		})
	})

	// Configure HTTP server with timeouts suitable for large file uploads.
	// Wrap the relay handler with a middleware that sets Content-Type headers
	// for blob GET requests from the MIME sidecar. Mirrored blobs have no
	// kind 24242 blob descriptor, so http.ServeContent can't infer the type
	// and falls back to application/octet-stream for formats Go can't sniff
	// (e.g. QuickTime .mov). Setting the header before ServeContent runs
	// causes it to respect the pre-set value.
	blossomPath := config.BlossomPath
	blobMimeMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only intercept GET/HEAD requests for bare sha256 paths
			// (/{64hex} or /{64hex}.{ext}) — same pattern the blossom
			// library uses to match blob GETs.
			if (r.Method == "GET" || r.Method == "HEAD") &&
				(len(r.URL.Path) == 65 || (strings.Index(r.URL.Path, ".") == 65)) &&
				strings.Index(r.URL.Path[1:], "/") == -1 &&
				blossomPath != nil {

				hash := strings.SplitN(r.URL.Path[1:], ".", 2)[0]
				if len(hash) == 64 {
					mimeMap := readMimeMap(*blossomPath)
					if mimeType, ok := mimeMap[strings.ToLower(hash)]; ok && mimeType != "" {
						w.Header().Set("Content-Type", mimeType)
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}

	// CORS override: khatru's built-in CORS uses wildcard origin and does not
	// allow credentials, so admin login requests with credentials: 'include'
	// get blocked by browsers. We wrap the response to echo the actual Origin
	// and set Access-Control-Allow-Credentials: true for cross-origin requests
	// that need cookies (e.g. /api/admin/*, /api/dashboard/*).
	handler := corsAllowCredentials(blobMimeMiddleware(relay))

	server := &http.Server{
		Addr:              ":" + config.RelayPort,
		Handler:           handler,
		ReadTimeout:       15 * time.Minute, // Increased to 15 minutes for very large files
		WriteTimeout:      15 * time.Minute, // Increased to 15 minutes
		IdleTimeout:       5 * time.Minute,  // Increased idle timeout
		ReadHeaderTimeout: 30 * time.Second, // Prevent slow header attacks
		MaxHeaderBytes:    1 << 20,          // 1MB max header size
	}

	fmt.Println("running on :" + config.RelayPort + " with extended timeouts for large uploads")
	server.ListenAndServe()
}

// corsAllowCredentials wraps a handler and overrides khatru's CORS headers
// for requests that include an Origin, so credential-bearing cross-origin
// requests (admin login / dashboard API) are accepted by browsers.
func corsAllowCredentials(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cw := &corsResponseWriter{ResponseWriter: w, r: r}
		next.ServeHTTP(cw, r)
	})
}

type corsResponseWriter struct {
	http.ResponseWriter
	r           *http.Request
	wroteHeader bool
}

func (w *corsResponseWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.setCORS()
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *corsResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *corsResponseWriter) setCORS() {
	if w.r.Header.Get("Upgrade") == "websocket" {
		return
	}
	origin := w.r.Header.Get("Origin")
	if origin == "" {
		return
	}
	allowed, err := isAllowedOrigin(origin, w.r.Host)
	if err != nil || !allowed {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	if vary := w.Header().Get("Vary"); vary == "" {
		w.Header().Set("Vary", "Origin")
	} else if !strings.Contains(vary, "Origin") {
		w.Header().Add("Vary", "Origin")
	}
}

// isAllowedOrigin permits the same eTLD+1 host as the incoming request host,
// ignoring port. This lets the CMS served on a different port (e.g. :8000)
// talk to /api while still rejecting arbitrary third-party sites.
func isAllowedOrigin(origin, host string) (bool, error) {
	u, err := url.Parse(origin)
	if err != nil {
		return false, err
	}
	if u.Scheme == "" || u.Host == "" {
		return false, nil
	}
	return strings.EqualFold(stripPort(u.Host), stripPort(host)), nil
}

func stripPort(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func (w *corsResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("response writer does not support Hijack")
}

func (w *corsResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func fetchNostrData(npubDomain string) {
	var body []byte
	var err error

	if npubDomain == "" {
		// Fall back to local file
		body, err = os.ReadFile("./public/.well-known/nostr.json")
		if err != nil {
			log.Printf("Error reading local nostr.json: %v", err)
			return
		}
		log.Println("Using local public/.well-known/nostr.json")
	} else {
		// Fetch from remote domain
		// First try /public/.well-known/nostr.json
		urls := []string{
			"https://" + npubDomain + "/public/.well-known/nostr.json",
			"https://" + npubDomain + "/.well-known/nostr.json",
		}

		var lastErr error
		for _, url := range urls {
			response, err := http.Get(url)
			if err != nil {
				lastErr = err
				continue
			}
			defer response.Body.Close()

			if response.StatusCode != http.StatusOK {
				lastErr = fmt.Errorf("HTTP %d", response.StatusCode)
				continue
			}

			body, err = io.ReadAll(response.Body)
			if err != nil {
				lastErr = err
				continue
			}

			// Basic JSON validation
			if len(body) > 0 && body[0] == '{' {
				log.Printf("Successfully fetched nostr.json from %s", url)
				lastErr = nil
				break
			}
			lastErr = fmt.Errorf("invalid JSON response from %s", url)
		}

		if lastErr != nil {
			log.Printf("Error fetching nostr.json from %s: %v", npubDomain, lastErr)
			return
		}
	}

	var newData NostrData
	err = json.Unmarshal(body, &newData)
	if err != nil {
		log.Printf("Error unmarshalling JSON: %v", err)
		return
	}

	data = newData
	for pubkey, names := range data.Names {
		fmt.Println(pubkey, names)
	}

	if npubDomain == "" {
		log.Println("Updated NostrData from local .well-known file")
	} else {
		log.Println("Updated NostrData from remote .well-known file")
	}
}

func LoadConfig() Config {
	// Load .env file if it exists, but don't overwrite existing environment variables
	// This allows docker-compose environment variables to take precedence
	if envMap, err := godotenv.Read(".env"); err == nil {
		for key, value := range envMap {
			if os.Getenv(key) == "" {
				os.Setenv(key, value)
			}
		}
	}

	config = Config{
		RelayName:             getEnv("RELAY_NAME"),
		RelayPubkey:           getEnv("RELAY_PUBKEY"),
		RelayDescription:      getEnv("RELAY_DESCRIPTION"),
		DBEngine:              getEnvNullable("DB_ENGINE"),
		DBPath:                getEnvNullable("DB_PATH"),
		PostgresUser:          getEnvNullable("POSTGRES_USER"),
		PostgresPassword:      getEnvNullable("POSTGRES_PASSWORD"),
		PostgresDB:            getEnvNullable("POSTGRES_DB"),
		PostgresHost:          getEnvNullable("POSTGRES_HOST"),
		PostgresPort:          getEnvNullable("POSTGRES_PORT"),
		DatabaseURL:           getEnvNullable("DATABASE_URL"),
		TeamDomain:            getEnvWithDefault("TEAM_DOMAIN", ""),
		NPUBDomain:            getEnvWithDefault("NPUB_DOMAIN", ""),
		BlossomEnabled:        getEnvBool("BLOSSOM_ENABLED"),
		BlossomPath:           getEnvWithDefaultPtr("BLOSSOM_PATH", "blossom/"),
		BlossomURL:            getEnvWithDefaultPtr("BLOSSOM_URL", "http://localhost:3334"),
		WebSocketURL:          getEnvWithDefaultPtr("WEBSOCKET_URL", "wss://localhost:3334"),
		AllowedKinds:          parseAllowedKinds(getEnvNullable("ALLOWED_KINDS")),
		PublicAllowedKinds:    parseAllowedKinds(getEnvNullable("PUBLIC_ALLOWED_KINDS")),
		TrustedClientName:     getEnvWithDefault("TRUSTED_CLIENT_NAME", ""),
		TrustedClientKinds:    parseTrustedClientKinds(getEnvNullable("TRUSTED_CLIENT_KINDS")),
		TrustedClientAllKinds: isTrustedClientAllKinds(getEnvNullable("TRUSTED_CLIENT_KINDS")),
		MaxUploadSizeMB:       getEnvIntWithDefault("MAX_UPLOAD_SIZE_MB", 200),
		RelayPort:             getEnvWithDefault("RELAY_PORT", "3334"),
		AllowedMirrorHosts:    parseAllowedMirrorHosts(getEnvNullable("ALLOWED_MIRROR_HOSTS")),
		// S3 Storage Configuration
		StorageBackend: getEnvWithDefault("STORAGE_BACKEND", "filesystem"),
		S3Endpoint:     getEnvWithDefault("S3_ENDPOINT", ""),
		S3Bucket:       getEnvWithDefault("S3_BUCKET", ""),
		S3Region:       getEnvWithDefault("S3_REGION", "auto"),
		S3PublicURL:    getEnvWithDefault("S3_PUBLIC_URL", ""),
	}

	relay.Info.Name = config.RelayName
	relay.Info.PubKey = config.RelayPubkey
	relay.Info.Description = config.RelayDescription
	if config.DBPath == nil {
		defaultPath := "db/"
		config.DBPath = &defaultPath
	}

	db = newDBBackend(*config.DBPath)

	if err := db.Init(); err != nil {
		panic(err)
	}

	fs = afero.NewOsFs()
	if config.BlossomEnabled {
		if config.StorageBackend == "s3" {
			// Initialize S3 storage
			s3Cfg := getS3ConfigFromEnv()
			if s3Cfg == nil {
				log.Fatalf("S3 storage backend selected but missing required environment variables (S3_ENDPOINT, S3_BUCKET, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)")
			}
			s3Cfg.ServiceURL = *config.BlossomURL
			var err error
			s3Storage, err = NewS3Storage(*s3Cfg)
			if err != nil {
				log.Fatalf("Failed to initialize S3 storage: %v", err)
			}
			log.Printf("Blossom using S3 storage backend: %s/%s", s3Cfg.Endpoint, s3Cfg.Bucket)
		} else {
			// Filesystem storage
			if config.BlossomPath == nil {
				log.Fatalf("Blossom enabled but no path set")
			}
			fs.MkdirAll(*config.BlossomPath, 0755)
			log.Printf("Blossom using filesystem storage backend: %s", *config.BlossomPath)
		}
	}

	return config
}

// Rate limiting data structures
type rateLimiter struct {
	mu       sync.RWMutex
	counters map[string][]time.Time
	limit    int
	window   time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		counters: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *rateLimiter) isAllowed(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Clean old entries
	times := rl.counters[key]
	var validTimes []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			validTimes = append(validTimes, t)
		}
	}

	// Check if under limit
	if len(validTimes) >= rl.limit {
		return false
	}

	// Add current request
	validTimes = append(validTimes, now)
	rl.counters[key] = validTimes

	return true
}

// Global rate limiters
var (
	pubkeyRateLimit *rateLimiter
	ipRateLimit     *rateLimiter
	connRateLimit   *rateLimiter
	queryRateLimit  *rateLimiter
)

// applySpamProtection applies rate limiting and spam protection policies
func applySpamProtection(relay *khatru.Relay, config Config) {
	pubkeyRateLimit = newRateLimiterFromEnv("PUBKEY_RATE_LIMIT", time.Minute)
	ipRateLimit = newRateLimiterFromEnv("IP_RATE_LIMIT", time.Minute)
	connRateLimit = newRateLimiterFromEnv("CONN_RATE_LIMIT", 2*time.Minute)
	queryRateLimit = newRateLimiterFromEnv("QUERY_RATE_LIMIT", time.Minute)

	// Rate limit events by pubkey (applies to all users)
	relay.RejectEvent = append(relay.RejectEvent, func(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
		// Check if user is team member (more lenient limits)
		isTeamMember := false
		for _, pubkey := range data.Names {
			if event.PubKey == pubkey {
				isTeamMember = true
				break
			}
		}

		// Apply stricter rate limits to non-team members
		if !isTeamMember {
			if pubkeyRateLimit != nil && !pubkeyRateLimit.isAllowed(event.PubKey) {
				return true, "rate-limited: too many events from this pubkey, slow down please"
			}
		}

		return false, ""
	})

	// Rate limit events by IP
	relay.RejectEvent = append(relay.RejectEvent, func(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
		ip := khatru.GetIP(ctx)
		if ip != "" && ipRateLimit != nil && !ipRateLimit.isAllowed(ip) {
			return true, "rate-limited: too many events from this IP, slow down please"
		}
		return false, ""
	})

	// Rate limit connections
	relay.RejectConnection = append(relay.RejectConnection, func(r *http.Request) bool {
		if connRateLimit == nil {
			return false
		}
		ip := khatru.GetIPFromRequest(r)
		return !connRateLimit.isAllowed(ip)
	})

	// Rate limit queries/filters
	relay.RejectFilter = append(relay.RejectFilter, func(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
		ip := khatru.GetIP(ctx)
		if ip != "" && queryRateLimit != nil && !queryRateLimit.isAllowed(ip) {
			return true, "rate-limited: too many queries from this IP"
		}
		return false, ""
	})

	// Reject events with base64 media (common spam vector)
	relay.RejectEvent = append(relay.RejectEvent, func(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
		if strings.Contains(event.Content, "data:image/") || strings.Contains(event.Content, "data:video/") {
			return true, "rejected: base64 media not allowed"
		}
		return false, ""
	})

	log.Println("Applied spam protection policies with configurable rate limiting")
	logRateLimiterConfig("PUBKEY_RATE_LIMIT", pubkeyRateLimit, "events/min per pubkey")
	logRateLimiterConfig("IP_RATE_LIMIT", ipRateLimit, "events/min per IP")
	logRateLimiterConfig("CONN_RATE_LIMIT", connRateLimit, "connections/2min per IP")
	logRateLimiterConfig("QUERY_RATE_LIMIT", queryRateLimit, "queries/min per IP")
}

func getEnv(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		log.Fatalf("Environment variable %s not set", key)
	}
	return value
}

func getEnvBool(key string) bool {
	value, exists := os.LookupEnv(key)
	if !exists {
		return false
	}
	return value == "true"
}

func getEnvNullable(key string) *string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return nil
	}
	return &value
}

func getEnvIntWithDefault(key string, defaultValue int) int {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("Warning: Invalid integer value '%s' for %s, using default %d", value, key, defaultValue)
		return defaultValue
	}
	return intValue
}

func getEnvOptionalInt(key string) *int {
	value, exists := os.LookupEnv(key)
	if !exists || strings.TrimSpace(value) == "" {
		return nil
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("Warning: Invalid integer value '%s' for %s, disabling this rate limiter", value, key)
		return nil
	}

	if intValue <= 0 {
		log.Printf("Warning: Non-positive value %d for %s, disabling this rate limiter", intValue, key)
		return nil
	}

	return &intValue
}

func newRateLimiterFromEnv(key string, window time.Duration) *rateLimiter {
	limit := getEnvOptionalInt(key)
	if limit == nil {
		return nil
	}
	return newRateLimiter(*limit, window)
}

func logRateLimiterConfig(envKey string, rl *rateLimiter, description string) {
	if rl == nil {
		log.Printf("%s not set: %s rate limit disabled", envKey, description)
		return
	}
	log.Printf("%s=%d: %s enabled", envKey, rl.limit, description)
}

// --- Blob owner map (sidecar file) ---
// Maps sha256 → owner pubkey so the /list/ endpoint can include owner
// info and the frontend can filter media by user.
// Stored as a JSON file at {blossom_path}/.metadata/owners.json

func getOwnerMapPath(blossomPath string) string {
	return blossomPath + ".metadata/owners.json"
}

// MIME type sidecar — sha256 → MIME type. Stored as JSON at
// {blossom_path}/.metadata/mimes.json. Used to set Content-Type headers
// for mirrored blobs that have no kind 24242 blob descriptor.
func getMimeMapPath(blossomPath string) string {
	return blossomPath + ".metadata/mimes.json"
}

func readMimeMap(blossomPath string) map[string]string {
	mimeMap := make(map[string]string)
	data, err := os.ReadFile(getMimeMapPath(blossomPath))
	if err != nil {
		return mimeMap
	}
	_ = json.Unmarshal(data, &mimeMap)
	return mimeMap
}

func setBlobMime(blossomPath string, sha256 string, mimeType string) {
	if mimeType == "" {
		return
	}
	mimeMap := readMimeMap(blossomPath)
	mimeMap[strings.ToLower(sha256)] = mimeType

	metaDir := blossomPath + ".metadata"
	_ = os.MkdirAll(metaDir, 0755)

	data, err := json.Marshal(mimeMap)
	if err != nil {
		log.Printf("Error marshaling mime map: %v", err)
		return
	}
	if err := os.WriteFile(getMimeMapPath(blossomPath), data, 0644); err != nil {
		log.Printf("Error writing mime map: %v", err)
	}
}

// getAdminRoles reads the adminRoles map from the most recent kind 30078
// site-config event published by the master pubkey. Returns an empty map
// if no config event is found or if the admin_roles tag is missing/invalid.
// This is the same data the frontend reads to determine publisher vs user.
func getAdminRoles(ctx context.Context, db DBBackend, masterPubkey string) map[string]string {
	roles := make(map[string]string)
	if masterPubkey == "" {
		return roles
	}
	ch, err := db.QueryEvents(ctx, nostr.Filter{
		Kinds:   []int{30078},
		Authors: []string{masterPubkey},
		Limit:   1,
	})
	if err != nil {
		return roles
	}
	for evt := range ch {
		for _, tag := range evt.Tags {
			if len(tag) >= 2 && tag[0] == "admin_roles" {
				if err := json.Unmarshal([]byte(tag[1]), &roles); err != nil {
					return roles
				}
				// Normalize keys to lowercase
				normalized := make(map[string]string)
				for k, v := range roles {
					normalized[strings.ToLower(k)] = v
				}
				return normalized
			}
		}
	}
	return roles
}

// readOwnerMap loads the sha256 → pubkey map from the sidecar file.
// Returns an empty map if the file doesn't exist.
func readOwnerMap(blossomPath string) map[string]string {
	ownerMap := make(map[string]string)
	data, err := os.ReadFile(getOwnerMapPath(blossomPath))
	if err != nil {
		return ownerMap
	}
	_ = json.Unmarshal(data, &ownerMap)
	return ownerMap
}

// setBlobOwner updates the sidecar file with a new sha256 → pubkey mapping.
// It reads the current file, adds/updates the entry, and writes it back.
func setBlobOwner(blossomPath string, sha256 string, pubkey string) {
	ownerMap := readOwnerMap(blossomPath)
	ownerMap[strings.ToLower(sha256)] = strings.ToLower(pubkey)

	// Ensure the .metadata directory exists
	metaDir := blossomPath + ".metadata"
	_ = os.MkdirAll(metaDir, 0755)

	data, err := json.Marshal(ownerMap)
	if err != nil {
		log.Printf("Error marshaling owner map: %v", err)
		return
	}
	if err := os.WriteFile(getOwnerMapPath(blossomPath), data, 0644); err != nil {
		log.Printf("Error writing owner map: %v", err)
	}
}

func getEnvWithDefaultPtr(key string, defaultValue string) *string {
	value, exists := os.LookupEnv(key)
	if !exists || strings.TrimSpace(value) == "" {
		return &defaultValue
	}
	return &value
}

func getEnvWithDefault(key string, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if !exists || strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

func parseAllowedKinds(allowedKindsStr *string) []int {
	if allowedKindsStr == nil || strings.TrimSpace(*allowedKindsStr) == "" {
		return []int{} // Empty slice means allow all kinds
	}

	kindsStr := strings.TrimSpace(*allowedKindsStr)
	kindStrings := strings.Split(kindsStr, ",")
	var kinds []int

	for _, kindStr := range kindStrings {
		kindStr = strings.TrimSpace(kindStr)
		if kindStr == "" {
			continue
		}

		kind, err := strconv.Atoi(kindStr)
		if err != nil {
			log.Printf("Warning: Invalid kind '%s' in ALLOWED_KINDS, skipping", kindStr)
			continue
		}
		kinds = append(kinds, kind)
	}

	if len(kinds) > 0 {
		log.Printf("Relay configured to only allow kinds: %v", kinds)
	} else {
		log.Printf("Relay configured to allow all kinds")
	}

	return kinds
}

func isTrustedClientAllKinds(kindsStr *string) bool {
	if kindsStr == nil {
		return false
	}
	return strings.TrimSpace(strings.ToLower(*kindsStr)) == "all"
}

func parseTrustedClientKinds(kindsStr *string) []int {
	if kindsStr == nil || strings.TrimSpace(*kindsStr) == "" {
		return []int{}
	}
	// If "all" is specified, return empty slice (TrustedClientAllKinds flag handles this)
	if strings.TrimSpace(strings.ToLower(*kindsStr)) == "all" {
		log.Println("Trusted client configured to allow ALL kinds")
		return []int{}
	}
	return parseAllowedKinds(kindsStr)
}

func parseAllowedMirrorHosts(hostsStr *string) []string {
	if hostsStr == nil || strings.TrimSpace(*hostsStr) == "" {
		return []string{} // Empty slice means mirror endpoint is disabled
	}

	hostsStrVal := strings.TrimSpace(*hostsStr)
	hostStrings := strings.Split(hostsStrVal, ",")
	var hosts []string

	for _, hostStr := range hostStrings {
		hostStr = strings.TrimSpace(hostStr)
		if hostStr == "" {
			continue
		}
		// Normalize: remove trailing slashes and convert to lowercase
		hostStr = strings.ToLower(strings.TrimRight(hostStr, "/"))
		hosts = append(hosts, hostStr)
	}

	if len(hosts) > 0 {
		log.Printf("Mirror endpoint enabled for hosts: %v", hosts)
	} else {
		log.Printf("Mirror endpoint disabled (no allowed hosts configured)")
	}

	return hosts
}

// urlRegex matches http/https URLs in text content.
var urlRegex = regexp.MustCompile(`https?://[^\s"'<>)\]]+`)

// hasMediaExtension reports whether the URL ends with a known media file extension.
func hasMediaExtension(url string) bool {
	// Strip query params and fragments
	if idx := strings.IndexAny(url, "?#"); idx >= 0 {
		url = url[:idx]
	}
	exts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif", ".bmp", ".svg",
		".mp4", ".webm", ".mov", ".qt", ".m4v", ".ogg", ".mp3", ".wav", ".pdf", ".heic", ".heif"}
	for _, ext := range exts {
		if strings.HasSuffix(url, ext) {
			return true
		}
	}
	return false
}

// isVideoExt reports whether the URL ends with a video file extension.
func isVideoExt(url string) bool {
	if idx := strings.IndexAny(url, "?#"); idx >= 0 {
		url = url[:idx]
	}
	exts := []string{".mp4", ".webm", ".mov", ".qt", ".m4v", ".ogg"}
	for _, ext := range exts {
		if strings.HasSuffix(url, ext) {
			return true
		}
	}
	return false
}

// detectMimeType determines the MIME type of a blob from its content.
// It handles QuickTime (.mov) and other ISO BMFF formats (MP4, M4V) by
// checking the "ftyp" atom at byte 4, which Go's built-in
// http.DetectContentType does not recognize. Falls back to the Go
// sniffer for all other formats.
func detectMimeType(body []byte) string {
	// ISO BMFF (MP4/QuickTime/M4V) files start with a size uint32
	// followed by "ftyp" and a 4-byte brand. Check for this pattern.
	if len(body) >= 12 && string(body[4:8]) == "ftyp" {
		brand := string(body[8:12])
		switch brand {
		case "qt  ":
			return "video/quicktime"
		case "mp41", "mp42", "isom", "iso2", "avc1", "mp71":
			return "video/mp4"
		case "M4V ", "M4VH", "M4VP":
			return "video/x-m4v"
		case "heic", "heix", "hevc", "hevx":
			return "image/heic"
		case "mif1":
			return "image/heif"
		default:
			// Unknown ftyp brand — still an ISO BMFF container, likely MP4
			return "video/mp4"
		}
	}

	// Fall back to Go's built-in content sniffer for everything else
	return http.DetectContentType(body)
}

// isHex checks if a string contains only hexadecimal characters.
func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// readBlossomAuth reads and validates a NIP-98 Authorization header from the
// request. Returns the authenticated event or nil if no valid auth is present.
func readBlossomAuth(r *http.Request) (*nostr.Event, error) {
	token := r.Header.Get("Authorization")
	if !strings.HasPrefix(token, "Nostr ") {
		return nil, nil
	}

	eventj, err := base64.StdEncoding.DecodeString(token[6:])
	if err != nil {
		return nil, fmt.Errorf("invalid base64 token")
	}

	var evt nostr.Event
	if err := json.Unmarshal(eventj, &evt); err != nil {
		return nil, fmt.Errorf("broken event")
	}
	if evt.Kind != 24242 || !evt.CheckID() {
		return nil, fmt.Errorf("invalid event")
	}
	valid, err := evt.CheckSignature()
	if err != nil || !valid {
		return nil, fmt.Errorf("invalid signature")
	}

	expirationTag := evt.Tags.GetFirst([]string{"expiration"})
	if expirationTag == nil {
		return nil, fmt.Errorf("missing expiration tag")
	}
	expiration, _ := strconv.ParseInt((*expirationTag)[1], 10, 64)
	if nostr.Timestamp(expiration) < nostr.Now() {
		return nil, fmt.Errorf("event expired")
	}

	return &evt, nil
}

// isAllowedMirrorURL validates that the URL is from an allowed host to prevent SSRF attacks.
// Use "*" in ALLOWED_MIRROR_HOSTS to allow any public http/https URL (harvest use case).
func isAllowedMirrorURL(rawURL string, allowedHosts []string) bool {
	if len(allowedHosts) == 0 {
		return false // No hosts configured means mirror is disabled
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	// Only allow http and https schemes (never file://, internal IPs via *, etc.)
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return false
	}

	host := strings.ToLower(parsedURL.Host)

	// Strip port for SSRF check
	hostOnly := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostOnly = h
	}

	for _, allowedHost := range allowedHosts {
		// Wildcard: allow any public host but block RFC-1918 / loopback addresses
		if allowedHost == "*" {
			if ip := net.ParseIP(hostOnly); ip != nil {
				if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
					return false
				}
			}
			// Block localhost by name
			if hostOnly == "localhost" {
				return false
			}
			return true
		}
		if host == allowedHost {
			return true
		}
	}

	return false
}

type DBBackend interface {
	Init() error
	Close()
	CountEvents(ctx context.Context, filter nostr.Filter) (int64, error)
	DeleteEvent(ctx context.Context, evt *nostr.Event) error
	QueryEvents(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error)
	SaveEvent(ctx context.Context, evt *nostr.Event) error
	ReplaceEvent(ctx context.Context, evt *nostr.Event) error
}

func newDBBackend(path string) DBBackend {
	if config.DBEngine == nil {
		defaultEngine := "postgres"
		config.DBEngine = &defaultEngine
	}

	switch *config.DBEngine {
	case "lmdb":
		return newLMDBBackend(path)
	case "badger":
		return &badger.BadgerBackend{
			Path:     path,
			MaxLimit: 50000, // allow full personal relay queries; default 1000 caps at 250/query
		}
	default:
		return newPostgresBackend()
	}
}

func newLMDBBackend(path string) *lmdb.LMDBBackend {
	return &lmdb.LMDBBackend{
		Path: path,
	}
}

func newPostgresBackend() DBBackend {
	var dbURL string
	if config.DatabaseURL != nil && *config.DatabaseURL != "" {
		dbURL = *config.DatabaseURL
		log.Println("Using DATABASE_URL for postgres connection")
	} else {
		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			*config.PostgresUser, *config.PostgresPassword, *config.PostgresHost, *config.PostgresPort, *config.PostgresDB)
		log.Println("Using individual POSTGRES_* variables for postgres connection")
	}
	return &postgresql.PostgresBackend{
		DatabaseURL: dbURL,
	}
}

// extractSha256FromURL extracts the SHA256 hash from a blossom URL
// Expected format: https://server.com/sha256hash or https://server.com/sha256hash.ext
func extractSha256FromURL(url string) string {
	// Remove the protocol and domain
	parts := strings.Split(url, "/")
	if len(parts) < 4 {
		return ""
	}

	// Get the last part which should be the hash (possibly with extension)
	hashPart := parts[len(parts)-1]

	// Remove file extension if present
	if dotIndex := strings.LastIndex(hashPart, "."); dotIndex != -1 {
		hashPart = hashPart[:dotIndex]
	}

	// Validate that it looks like a SHA256 hash (64 hex characters)
	if len(hashPart) == 64 {
		// Check if all characters are valid hex
		for _, char := range hashPart {
			if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
				return ""
			}
		}
		return strings.ToLower(hashPart)
	}

	return ""
}

func setupConvertHandlers(relay *khatru.Relay, config Config) {
	// Serve the NIP-05 registration page
	relay.Router().HandleFunc("/convert", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.ServeFile(w, r, "./public/convert.html")
	})
}

// Helper functions for NIP-05 validation
func matchUsernamePattern(username string) bool {
	if len(username) < 1 || len(username) > 64 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-z0-9_\.\-]+$`, username)
	return matched
}

func validateAndConvertPubkey(input string) (string, error) {
	input = strings.TrimSpace(input)

	// Check if it's a valid 64-character hex string
	if matched, _ := regexp.MatchString(`^[0-9a-fA-F]{64}$`, input); matched {
		return strings.ToLower(input), nil
	}

	// If it's not hex, try to convert from npub (fallback)
	if strings.HasPrefix(input, "npub1") {
		// Use nostr-tools for npub conversion
		// We'll implement a simple bech32 decoder for npub
		hexKey, err := decodeNpub(input)
		if err != nil {
			return "", fmt.Errorf("invalid npub format: %v", err)
		}
		return hexKey, nil
	}

	return "", fmt.Errorf("invalid public key format - must be npub1... or 64-character hex")
}

// decodeNpub converts npub1... format to hex string
func decodeNpub(npub string) (string, error) {
	if len(npub) != 63 {
		return "", fmt.Errorf("invalid npub length")
	}

	// Remove the "npub1" prefix
	data := npub[5:]

	// Decode from bech32
	converted, err := bech32Decode(data)
	if err != nil {
		return "", err
	}

	// Convert from 5-bit groups to 8-bit groups
	result, err := convertBits(converted, 5, 8, false)
	if err != nil {
		return "", err
	}

	// Verify length (should be 32 bytes for pubkey)
	if len(result) != 32 {
		return "", fmt.Errorf("invalid decoded data length")
	}

	// Convert to hex string
	return fmt.Sprintf("%x", result), nil
}

// Simple bech32 decoder implementation
func bech32Decode(s string) ([]byte, error) {
	// Bech32 character set
	charset := "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

	// Convert string to byte indices
	var converted []byte
	for _, char := range s {
		index := strings.IndexByte(charset, byte(char))
		if index == -1 {
			return nil, fmt.Errorf("invalid character in bech32 string")
		}
		converted = append(converted, byte(index))
	}

	return converted, nil
}

// ConvertBits converts from bit groups of size fromBits to bit groups of size toBits
func convertBits(data []byte, fromBits, toBits uint, pad bool) ([]byte, error) {
	acc := uint(0)
	bits := uint(0)
	var result []byte
	maxv := uint((1 << toBits) - 1)
	maxAcc := uint((1 << (fromBits + toBits - 1)) - 1)

	for i := 0; i < len(data); i++ {
		value := uint(data[i])
		if (value >> fromBits) != 0 {
			return nil, fmt.Errorf("invalid data range")
		}

		acc = ((acc << fromBits) | value) & maxAcc
		bits += fromBits

		for bits >= toBits {
			bits -= toBits
			result = append(result, byte((acc>>bits)&maxv))
		}
	}

	if pad {
		if bits > 0 {
			result = append(result, byte((acc<<(toBits-bits))&maxv))
		}
	} else if bits >= fromBits || ((acc<<(toBits-bits))&maxv) != 0 {
		return nil, fmt.Errorf("invalid padding")
	}

	return result, nil
}

// updateNostrJson updates the nostr.json file in persistent volume
func updateNostrJson(username, pubkey string) error {
	// Use environment variable or default to local path
	nostrJsonPath := getEnvWithDefault("NIP05_PATH", "public/.well-known/nostr.json")

	// If running in Docker, convert relative path to absolute
	if !filepath.IsAbs(nostrJsonPath) && os.Getenv("DOCKER_ENV") == "true" {
		nostrJsonPath = "/app/" + nostrJsonPath
	}

	// Read existing file
	var nostrData map[string]interface{}

	// Create file if it doesn't exist
	if _, err := os.Stat(nostrJsonPath); os.IsNotExist(err) {
		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(nostrJsonPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %s", err)
		}

		// Initialize with empty structure
		nostrData = map[string]interface{}{
			"names": map[string]interface{}{},
		}
	} else {
		// Read existing file
		data, err := os.ReadFile(nostrJsonPath)
		if err != nil {
			return fmt.Errorf("failed to read nostr.json: %s", err)
		}

		if err := json.Unmarshal(data, &nostrData); err != nil {
			return fmt.Errorf("failed to parse nostr.json: %s", err)
		}
	}

	// Ensure names object exists
	if nostrData["names"] == nil {
		nostrData["names"] = map[string]interface{}{}
	}

	names, ok := nostrData["names"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid nostr.json structure: names is not an object")
	}

	// Check if username already exists
	if existingPubkey, exists := names[username]; exists {
		if existingPubkey == pubkey {
			return fmt.Errorf("username %s is already registered with this pubkey", username)
		}
		return fmt.Errorf("username %s is already registered with a different pubkey", username)
	}

	// Check if pubkey is already used by another username
	for existingUser, existingPubkey := range names {
		if existingPubkey == pubkey {
			return fmt.Errorf("pubkey is already registered to username %s", existingUser)
		}
	}

	// Add new entry
	names[username] = pubkey
	nostrData["names"] = names

	// Write back to file
	updatedData, err := json.MarshalIndent(nostrData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal nostr.json: %s", err)
	}

	if err := os.WriteFile(nostrJsonPath, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write nostr.json: %s", err)
	}

	log.Printf("Successfully added NIP-05 entry: %s -> %s", username, pubkey)
	return nil
}

// initializeNostrJson creates nostr.json with relay pubkey as root if it doesn't exist
func initializeNostrJson(config Config) error {
	nostrJsonPath := getEnvWithDefault("NIP05_PATH", "public/.well-known/nostr.json")

	// If running in Docker, convert relative path to absolute
	if !filepath.IsAbs(nostrJsonPath) && os.Getenv("DOCKER_ENV") == "true" {
		nostrJsonPath = "/app/" + nostrJsonPath
	}

	// Check if file already exists
	if _, err := os.Stat(nostrJsonPath); err == nil {
		// File exists, check if it has root entry
		data, err := os.ReadFile(nostrJsonPath)
		if err != nil {
			return fmt.Errorf("failed to read existing nostr.json: %s", err)
		}

		var nostrData map[string]interface{}
		if err := json.Unmarshal(data, &nostrData); err != nil {
			return fmt.Errorf("failed to parse existing nostr.json: %s", err)
		}

		// Check if root entry exists
		if names, ok := nostrData["names"].(map[string]interface{}); ok {
			if _, hasRoot := names["_"]; hasRoot {
				log.Println("Root entry already exists in nostr.json")
				return nil
			}
		}
	}

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(nostrJsonPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %s", err)
	}

	// Create nostr.json with relay pubkey as root
	nostrData := map[string]interface{}{
		"names": map[string]interface{}{
			"_": config.RelayPubkey,
		},
	}

	updatedData, err := json.MarshalIndent(nostrData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal nostr.json: %s", err)
	}

	if err := os.WriteFile(nostrJsonPath, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write nostr.json: %s", err)
	}

	log.Printf("Initialized nostr.json with root entry: _ -> %s", config.RelayPubkey)
	return nil
}

// setupDashboardHandlers adds all the API endpoints for the dashboard
func setupDashboardHandlers(relay *khatru.Relay, config Config) {
	// Serve dashboard HTML
	relay.Router().HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.ServeFile(w, r, "./public/dashboard.html")
	})

	registerAdminAPI := func(path string, handler http.HandlerFunc) {
		relay.Router().HandleFunc("/api/dashboard"+path, handler)
		relay.Router().HandleFunc("/api/admin"+path, handler)
	}

	// isKnownTeamPubkey reports whether the given hex pubkey is listed in
	// nostr.json (any entry, including "_"). This is the relay-side
	// definition of "team member" — it does NOT know about the frontend's
	// primary/secondary distinction, which lives in a kind-30078 site-config
	// event the relay does not read.
	isKnownTeamPubkey := func(pk string) bool {
		normalized := strings.ToLower(strings.TrimSpace(pk))
		if normalized == "" {
			return false
		}
		for _, known := range data.Names {
			if strings.ToLower(strings.TrimSpace(known)) == normalized {
				return true
			}
		}
		return false
	}

	// requireTeamSession accepts any pubkey listed in nostr.json.
	// Used for read-only dashboard endpoints (stats, events, UI) that
	// should be reachable by any team member who authenticates.
	requireTeamSession := func(w http.ResponseWriter, r *http.Request) bool {
		cookie, err := r.Cookie("dashboard_session")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return false
		}

		if !isKnownTeamPubkey(cookie.Value) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return false
		}

		return true
	}

	// requireOwnerSession accepts only the relay operator ("_" in nostr.json).
	// Used for destructive / management endpoints (add/update/delete users).
	requireOwnerSession := func(w http.ResponseWriter, r *http.Request) bool {
		cookie, err := r.Cookie("dashboard_session")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return false
		}

		adminPubkey := resolveDashboardAdminPubkey(config)
		if strings.ToLower(strings.TrimSpace(cookie.Value)) != strings.ToLower(strings.TrimSpace(adminPubkey)) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return false
		}

		return true
	}

	// API: Login endpoint
	registerAdminAPI("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Pubkey string `json:"pubkey"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		adminPubkey := resolveDashboardAdminPubkey(config)

		// Accept any pubkey listed in nostr.json (team member).
		// The frontend's primary/secondary distinction is enforced
		// client-side via adminRoles; the relay only checks team membership.
		if !isKnownTeamPubkey(req.Pubkey) {
			log.Printf("Dashboard login denied: req pubkey %s... not in nostr.json", truncatePubkey(req.Pubkey))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Access denied. Only team members listed in nostr.json can login."})
			return
		}

		// Set a simple session cookie for the UI endpoint
		http.SetCookie(w, &http.Cookie{
			Name:     "dashboard_session",
			Value:    req.Pubkey,
			Path:     "/",
			HttpOnly: true,
			Secure:   os.Getenv("DOCKER_ENV") == "true",
			SameSite: http.SameSiteLaxMode,
			MaxAge:   3600, // 1 hour
		})

		// Determine if this login is the relay owner ("_" in nostr.json)
		isOwner := strings.EqualFold(strings.TrimSpace(req.Pubkey), strings.TrimSpace(adminPubkey))

		// Return dashboard data
		response := map[string]interface{}{
			"relayName":        config.RelayName,
			"relayDescription": config.RelayDescription,
			"users":            data.Names,
			"environment":      getEnvironmentVars(),
			"isRemote":         config.NPUBDomain != "",
			"npubDomain":       config.NPUBDomain,
			"pubkey":           req.Pubkey,
			"isOwner":          isOwner,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// API: Get dashboard UI
	registerAdminAPI("/ui", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !requireTeamSession(w, r) {
			return
		}

		http.ServeFile(w, r, "./templates/dashboard_view.html")
	})

	// API: Logout endpoint
	registerAdminAPI("/logout", func(w http.ResponseWriter, r *http.Request) {
		// Clear session cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "dashboard_session",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1,
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	})

	// API: Users endpoint (GET for list, POST for add)
	registerAdminAPI("/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			response := map[string]interface{}{
				"users":      data.Names,
				"isRemote":   config.NPUBDomain != "",
				"npubDomain": config.NPUBDomain,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)

		case "POST":
			if !requireOwnerSession(w, r) {
				return
			}

			// Only allow if using local nostr.json
			if config.NPUBDomain != "" {
				http.Error(w, "Cannot modify users when using remote nostr.json", http.StatusForbidden)
				return
			}

			var req struct {
				Name   string `json:"name"`
				Pubkey string `json:"pubkey"`
			}

			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}

			// Validate inputs
			if req.Name == "" || req.Pubkey == "" {
				http.Error(w, "Name and pubkey are required", http.StatusBadRequest)
				return
			}

			// Validate pubkey format (64 hex chars)
			if len(req.Pubkey) != 64 || !isValidHex(req.Pubkey) {
				http.Error(w, "Invalid pubkey format", http.StatusBadRequest)
				return
			}

			// Add user to local nostr.json
			if err := addOrUpdateUser(req.Name, req.Pubkey); err != nil {
				http.Error(w, "Failed to add user: "+err.Error(), http.StatusInternalServerError)
				return
			}

			// Refresh data
			fetchNostrData("")

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "success"})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
	})

	// API: Individual user operations (PUT for update, DELETE for delete)
	registerAdminAPI("/user/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" || r.Method == "DELETE" {
			if !requireOwnerSession(w, r) {
				return
			}
		}

		// Only allow if using local nostr.json
		if config.NPUBDomain != "" {
			http.Error(w, "Cannot modify users when using remote nostr.json", http.StatusForbidden)
			return
		}

		// Extract pubkey from URL
		pubkey := strings.TrimPrefix(r.URL.Path, "/api/dashboard/user/")
		if pubkey == r.URL.Path {
			pubkey = strings.TrimPrefix(r.URL.Path, "/api/admin/user/")
		}
		if pubkey == "" {
			http.Error(w, "Missing pubkey", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case "PUT":
			var req struct {
				Name string `json:"name"`
			}

			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}

			if req.Name == "" {
				http.Error(w, "Name is required", http.StatusBadRequest)
				return
			}

			// Update user in local nostr.json
			if err := addOrUpdateUser(req.Name, pubkey); err != nil {
				http.Error(w, "Failed to update user: "+err.Error(), http.StatusInternalServerError)
				return
			}

		case "DELETE":
			// Don't allow deleting the root entry
			if strings.EqualFold(strings.TrimSpace(pubkey), strings.TrimSpace(resolveDashboardAdminPubkey(config))) {
				http.Error(w, "Cannot delete root entry", http.StatusForbidden)
				return
			}

			// Delete user from local nostr.json
			if err := deleteUser(pubkey); err != nil {
				http.Error(w, "Failed to delete user: "+err.Error(), http.StatusInternalServerError)
				return
			}

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Refresh data
		fetchNostrData("")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	})

	// API: Get environment variables
	registerAdminAPI("/environment", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		response := map[string]interface{}{
			"environment": getEnvironmentVars(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// API: Convert pubkey
	registerAdminAPI("/convert", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Input string `json:"input"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		var result map[string]interface{}
		if strings.HasPrefix(req.Input, "npub1") {
			// Convert npub to hex
			hex, err := npubToHex(req.Input)
			if err != nil {
				result = map[string]interface{}{"error": "Invalid npub format: " + err.Error()}
			} else {
				result = map[string]interface{}{"hex": hex}
			}
		} else if len(req.Input) == 64 && isValidHex(req.Input) {
			// Convert hex to npub
			npub, err := hexToNpub(req.Input)
			if err != nil {
				result = map[string]interface{}{"error": "Invalid hex format: " + err.Error()}
			} else {
				result = map[string]interface{}{"npub": npub}
			}
		} else {
			result = map[string]interface{}{"error": "Invalid pubkey format. Expected 64-char hex or npub1..."}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// API: Personal stats for the authenticated user.
	// Scans the full DB (like /stats) but filters to the caller's pubkey.
	// This is more accurate than a client-side Nostr query because:
	//   - Kind 24242 (Blossom blob index) events are not in the pubkey
	//     index and can't be queried by `authors` via WebSocket
	//   - The badger pubkey-only index has a limit bug that returns fewer
	//     events than requested when no `kinds` filter is specified
	registerAdminAPI("/my-stats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cookie, err := r.Cookie("dashboard_session")
		if err != nil || !isKnownTeamPubkey(cookie.Value) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userPubkey := strings.ToLower(strings.TrimSpace(cookie.Value))

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Scan all events and tally only those belonging to the caller.
		// On a personal relay this is fast enough (typically < 20k events).
		ch, err := db.QueryEvents(ctx, nostr.Filter{Limit: 0})
		if err != nil {
			log.Printf("my-stats: QueryEvents error: %v", err)
			http.Error(w, "Failed to query events", http.StatusInternalServerError)
			return
		}

		byKind := make(map[int]int64)
		var total int64
		var lastActivity int64
		embeddedImages := 0
		embeddedVideos := 0

		for evt := range ch {
			if evt.PubKey != userPubkey {
				continue
			}
			total++
			byKind[evt.Kind]++

			if evt.CreatedAt.Time().Unix() > lastActivity {
				lastActivity = evt.CreatedAt.Time().Unix()
			}

			// Scan imeta tags (NIP-92) for embedded media in any event kind
			for _, tag := range evt.Tags {
				if len(tag) < 2 || tag[0] != "imeta" {
					continue
				}
				// imeta is a multi-line tag: ["imeta", "url https://...", "m image/jpeg", ...]
				var url, mime string
				for _, s := range tag[1:] {
					if len(s) > 4 && s[:4] == "url " {
						url = s[4:]
					} else if len(s) > 2 && s[:2] == "m " {
						mime = s[2:]
					}
				}
				if url == "" {
					continue
				}
				// Use MIME type if available, otherwise fall back to URL extension
				isVideo := false
				isImage := false
				if mime != "" {
					isVideo = strings.HasPrefix(mime, "video/")
					isImage = strings.HasPrefix(mime, "image/")
				} else {
					isVideo = strings.HasSuffix(strings.ToLower(url), ".mp4") ||
						strings.HasSuffix(strings.ToLower(url), ".webm") ||
						strings.HasSuffix(strings.ToLower(url), ".ogg") ||
						strings.HasSuffix(strings.ToLower(url), ".mov") ||
						strings.HasSuffix(strings.ToLower(url), ".m4v")
					isImage = strings.HasSuffix(strings.ToLower(url), ".png") ||
						strings.HasSuffix(strings.ToLower(url), ".jpg") ||
						strings.HasSuffix(strings.ToLower(url), ".jpeg") ||
						strings.HasSuffix(strings.ToLower(url), ".gif") ||
						strings.HasSuffix(strings.ToLower(url), ".webp") ||
						strings.HasSuffix(strings.ToLower(url), ".bmp") ||
						strings.HasSuffix(strings.ToLower(url), ".svg")
				}
				if isVideo {
					embeddedVideos++
				} else if isImage {
					embeddedImages++
				}
			}

			// Also scan image/thumb/banner/picture tags (kind 0, kind 30023)
			for _, tag := range evt.Tags {
				if len(tag) >= 2 && (tag[0] == "image" || tag[0] == "thumb" || tag[0] == "banner" || tag[0] == "picture") {
					url := strings.ToLower(tag[1])
					if hasMediaExtension(url) {
						if isVideoExt(url) {
							embeddedVideos++
						} else {
							embeddedImages++
						}
					}
				}
			}

			// Scan raw content for media URLs (many users embed image URLs
			// in text notes without NIP-92 imeta tags). This matches the
			// harvest's extractMediaUrlsFromEvent logic.
			if evt.Kind != 24242 && evt.Content != "" {
				// For kind 0 (profile), parse JSON and check picture/banner/image fields
				if evt.Kind == 0 {
					var profile map[string]interface{}
					if json.Unmarshal([]byte(evt.Content), &profile) == nil {
						for _, field := range []string{"picture", "banner", "image"} {
							if val, ok := profile[field].(string); ok {
								url := strings.ToLower(val)
								if hasMediaExtension(url) {
									if isVideoExt(url) {
										embeddedVideos++
									} else {
										embeddedImages++
									}
								}
							}
						}
					}
				} else {
					// Scan content text for URLs with media extensions
					lower := strings.ToLower(evt.Content)
					for _, url := range urlRegex.FindAllString(lower, -1) {
						// Strip trailing punctuation
						url = strings.TrimRight(url, ".,;:!?)]")
						if hasMediaExtension(url) {
							if isVideoExt(url) {
								embeddedVideos++
							} else {
								embeddedImages++
							}
						}
					}
				}
			}
		}

		// Count actual blobs on disk owned by this user (not kind 24242
		// events, which are only created on direct Blossom uploads — the
		// harvest stores blobs without creating 24242 events).
		blossomCount := 0
		blossomTotalSize := 0
		blossomImages := 0
		blossomVideos := 0
		blossomOther := 0
		if config.BlossomPath != nil {
			ownerMap := readOwnerMap(*config.BlossomPath)
			file, err := fs.Open(*config.BlossomPath)
			if err == nil {
				fileInfos, err := file.Readdir(-1)
				if err == nil {
					for _, fileInfo := range fileInfos {
						if fileInfo.IsDir() {
							continue
						}
						fileName := fileInfo.Name()
						if len(fileName) != 64 {
							continue
						}
						// Check if this blob belongs to the user
						sha := strings.ToLower(fileName)
						owner, ok := ownerMap[sha]
						if !ok || owner != userPubkey {
							continue
						}
						blossomCount++
						blossomTotalSize += int(fileInfo.Size())

						// Detect MIME type
						filePath := *config.BlossomPath + fileName
						contentType := "application/octet-stream"
						if blobFile, err := fs.Open(filePath); err == nil {
							buffer := make([]byte, 512)
							if n, err := blobFile.Read(buffer); err == nil && n > 0 {
								detectedType := http.DetectContentType(buffer[:n])
								if detectedType != "" {
									contentType = detectedType
								}
							}
							blobFile.Close()
						}
						if strings.HasPrefix(contentType, "image/") {
							blossomImages++
						} else if strings.HasPrefix(contentType, "video/") {
							blossomVideos++
						} else {
							blossomOther++
						}
					}
				}
				file.Close()
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pubkey":       userPubkey,
			"total":        total,
			"byKind":       byKind,
			"lastActivity": lastActivity,
			"blossom": map[string]interface{}{
				"count":     blossomCount,
				"totalSize": blossomTotalSize,
				"images":    blossomImages,
				"videos":    blossomVideos,
				"other":     blossomOther,
			},
			"embedded": map[string]interface{}{
				"images": embeddedImages,
				"videos": embeddedVideos,
			},
		})
	})

	// API: Relay stats — total events, per-kind, per-pubkey breakdown
	registerAdminAPI("/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !requireTeamSession(w, r) {
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Total event count
		total, err := db.CountEvents(ctx, nostr.Filter{})
		if err != nil {
			log.Printf("stats: CountEvents error: %v", err)
			http.Error(w, "Failed to count events", http.StatusInternalServerError)
			return
		}

		// Stream all events to aggregate by kind and pubkey.
		// badger has no native grouping, so we scan once and tally in memory.
		ch, err := db.QueryEvents(ctx, nostr.Filter{Limit: 0})
		if err != nil {
			log.Printf("stats: QueryEvents error: %v", err)
			http.Error(w, "Failed to query events", http.StatusInternalServerError)
			return
		}

		byKind := make(map[int]int64)
		byPubkey := make(map[string]int64)

		for evt := range ch {
			byKind[evt.Kind]++
			byPubkey[evt.PubKey]++
		}

		// Build a list of known pubkeys from nostr.json for the response.
		// The "_" entry marks the owner — we track its pubkey separately so
		// the frontend can show "Primary (Owner)" while still displaying the
		// user's real name (e.g. "buttercup") rather than "_".
		knownPubkeys := make(map[string]string) // hex → name
		ownerPubkey := ""
		for name, pk := range data.Names {
			hex := strings.ToLower(strings.TrimSpace(pk))
			if name == "_" {
				ownerPubkey = hex
				// Don't store "_" as the display name — a real alias may follow
			} else {
				knownPubkeys[hex] = name
			}
		}
		// If the owner pubkey has no other alias, fall back to "_"
		if ownerPubkey != "" {
			if _, hasName := knownPubkeys[ownerPubkey]; !hasName {
				knownPubkeys[ownerPubkey] = "_"
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"totalEvents":   total,
			"uniquePubkeys": len(byPubkey),
			"byKind":        byKind,
			"byPubkey":      byPubkey,
			"knownPubkeys":  knownPubkeys,
			"ownerPubkey":   ownerPubkey,
		})
	})

	// API: Browse events with filters and pagination
	registerAdminAPI("/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !requireTeamSession(w, r) {
			return
		}

		q := r.URL.Query()

		filter := nostr.Filter{}

		// Support both "kind" (single) and "kinds" (comma-separated) params.
		// "kinds" takes priority if both are present.
		if kindsStr := q.Get("kinds"); kindsStr != "" {
			for _, kStr := range strings.Split(kindsStr, ",") {
				if kind, err := strconv.Atoi(strings.TrimSpace(kStr)); err == nil {
					filter.Kinds = append(filter.Kinds, kind)
				}
			}
		} else if kindStr := q.Get("kind"); kindStr != "" {
			if kind, err := strconv.Atoi(kindStr); err == nil {
				filter.Kinds = []int{kind}
			}
		}

		if author := q.Get("author"); author != "" {
			filter.Authors = []string{author}
		}

		if sinceStr := q.Get("since"); sinceStr != "" {
			if since, err := strconv.ParseInt(sinceStr, 10, 64); err == nil {
				ts := nostr.Timestamp(since)
				filter.Since = &ts
			}
		}

		if untilStr := q.Get("until"); untilStr != "" {
			if until, err := strconv.ParseInt(untilStr, 10, 64); err == nil {
				ts := nostr.Timestamp(until)
				filter.Until = &ts
			}
		}

		limit := 50
		if limitStr := q.Get("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
				limit = l
			}
		}
		filter.Limit = limit

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		ch, err := db.QueryEvents(ctx, filter)
		if err != nil {
			log.Printf("events browse: QueryEvents error: %v", err)
			http.Error(w, "Failed to query events", http.StatusInternalServerError)
			return
		}

		type eventSummary struct {
			ID        string     `json:"id"`
			PubKey    string     `json:"pubkey"`
			Kind      int        `json:"kind"`
			CreatedAt int64      `json:"created_at"`
			Content   string     `json:"content"`
			Tags      nostr.Tags `json:"tags"`
		}

		events := make([]eventSummary, 0, limit)
		for evt := range ch {
			content := evt.Content
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			events = append(events, eventSummary{
				ID:        evt.ID,
				PubKey:    evt.PubKey,
				Kind:      evt.Kind,
				CreatedAt: int64(evt.CreatedAt),
				Content:   content,
				Tags:      evt.Tags,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"events": events,
			"count":  len(events),
			"limit":  limit,
		})
	})

	// API: Community zap stats — aggregate analytics for all community members.
	// Served from a pre-computed background snapshot (refreshed every 10 min).
	registerAdminAPI("/zap-stats", handleZapStats(requireTeamSession))
}

// getEnvironmentVars returns a map of environment variables (excluding sensitive ones)
func getEnvironmentVars() map[string]string {
	envVars := make(map[string]string)

	// List of environment variables to show (excluding sensitive ones)
	showVars := []string{
		"RELAY_NAME", "RELAY_PUBKEY", "RELAY_DESCRIPTION",
		"TEAM_DOMAIN", "NPUB_DOMAIN", "DB_ENGINE", "DB_PATH",
		"POSTGRES_HOST", "POSTGRES_PORT", "POSTGRES_DB", "POSTGRES_PASSWORD",
		"DATABASE_URL", "BLOSSOM_ENABLED", "BLOSSOM_PATH",
		"BLOSSOM_URL", "WEBSOCKET_URL", "ALLOWED_KINDS",
		"PUBLIC_ALLOWED_KINDS", "TRUSTED_CLIENT_NAME", "TRUSTED_CLIENT_KINDS",
		"MAX_UPLOAD_SIZE_MB", "RELAY_PORT", "ALLOWED_MIRROR_HOSTS",
		"STORAGE_BACKEND", "S3_ENDPOINT", "S3_BUCKET", "S3_REGION",
		"S3_PUBLIC_URL", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "NIP05_PATH",
	}

	for _, varName := range showVars {
		value := os.Getenv(varName)
		if value != "" {
			// Mask sensitive values
			if strings.Contains(strings.ToLower(varName), "password") ||
				strings.Contains(strings.ToLower(varName), "secret") ||
				strings.Contains(strings.ToLower(varName), "key") ||
				strings.Contains(strings.ToLower(varName), "database_url") {
				envVars[varName] = "***MASKED***"
			} else {
				envVars[varName] = value
			}
		} else {
			envVars[varName] = ""
		}
	}

	return envVars
}

// addOrUpdateUser adds or updates a user in the local nostr.json
func addOrUpdateUser(name, pubkey string) error {
	nostrJsonPath := "./public/.well-known/nostr.json"

	// Read existing file
	var nostrData map[string]interface{}
	if body, err := os.ReadFile(nostrJsonPath); err == nil {
		if err := json.Unmarshal(body, &nostrData); err != nil {
			return fmt.Errorf("failed to parse existing nostr.json: %s", err)
		}
	} else {
		// Create new structure if file doesn't exist
		nostrData = map[string]interface{}{
			"names": map[string]interface{}{},
		}
	}

	// Ensure names map exists
	names, ok := nostrData["names"].(map[string]interface{})
	if !ok {
		names = map[string]interface{}{}
		nostrData["names"] = names
	}

	// Add or update user
	names[name] = pubkey

	// Write back to file
	updatedData, err := json.MarshalIndent(nostrData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal nostr.json: %s", err)
	}

	if err := os.WriteFile(nostrJsonPath, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write nostr.json: %s", err)
	}

	log.Printf("Added/updated user: %s -> %s", name, pubkey)
	return nil
}

// deleteUser removes a user from the local nostr.json
func deleteUser(pubkey string) error {
	nostrJsonPath := "./public/.well-known/nostr.json"

	// Read existing file
	var nostrData map[string]interface{}
	body, err := os.ReadFile(nostrJsonPath)
	if err != nil {
		return fmt.Errorf("failed to read nostr.json: %s", err)
	}

	if err := json.Unmarshal(body, &nostrData); err != nil {
		return fmt.Errorf("failed to parse nostr.json: %s", err)
	}

	// Get names map
	names, ok := nostrData["names"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid nostr.json structure")
	}

	// Find and remove user with matching pubkey
	var userToDelete string
	for name, pk := range names {
		if pk == pubkey {
			userToDelete = name
			break
		}
	}

	if userToDelete == "" {
		return fmt.Errorf("user not found")
	}

	delete(names, userToDelete)

	// Write back to file
	updatedData, err := json.MarshalIndent(nostrData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal nostr.json: %s", err)
	}

	if err := os.WriteFile(nostrJsonPath, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write nostr.json: %s", err)
	}

	log.Printf("Deleted user: %s", userToDelete)
	return nil
}

// npubToHex converts npub format to hex using bech32 decoding
func npubToHex(npub string) (string, error) {
	if len(npub) < 5 || !strings.HasPrefix(npub, "npub1") {
		return "", fmt.Errorf("invalid npub format: must start with npub1")
	}

	// Simple bech32 validation and decoding
	// This is a simplified implementation - in production you'd use a proper bech32 library
	const alphabet = "023456789acdefghjklmnpqrstuvwxyz"

	// Remove prefix and convert to lowercase
	encoded := strings.ToLower(npub[5:])

	// Validate characters
	for _, c := range encoded {
		if !strings.ContainsRune(alphabet, c) {
			return "", fmt.Errorf("invalid character in npub")
		}
	}

	// Convert to 5-bit groups
	var data []byte
	for _, c := range encoded {
		index := strings.IndexRune(alphabet, c)
		if index == -1 {
			return "", fmt.Errorf("invalid character in npub")
		}

		// Convert to 5 bits
		for i := 4; i >= 0; i-- {
			bit := byte((index >> uint(i)) & 1)
			data = appendBits(data, bit)
		}
	}

	// Remove checksum (last 6 bits = checksum)
	if len(data) < 6 {
		return "", fmt.Errorf("npub too short")
	}
	data = data[:len(data)-6]

	// Convert bits to bytes
	var bytes []byte
	for i := 0; i < len(data); i += 8 {
		if i+8 <= len(data) {
			var b byte
			for j := 0; j < 8; j++ {
				if i+j < len(data) {
					b |= data[i+j] << uint(7-j)
				}
			}
			bytes = append(bytes, b)
		}
	}

	if len(bytes) < 32 {
		return "", fmt.Errorf("decoded data too short")
	}

	return hex.EncodeToString(bytes[:32]), nil
}

// appendBits appends a bit to a byte slice
func appendBits(data []byte, bit byte) []byte {
	byteIndex := len(data) / 8
	bitIndex := len(data) % 8

	if bitIndex == 0 {
		data = append(data, 0)
	}

	data[byteIndex] |= bit << uint(7-bitIndex)
	return data
}

// hexToNpub converts hex to npub format using bech32 encoding
func hexToNpub(hexStr string) (string, error) {
	if len(hexStr) != 64 || !isValidHex(hexStr) {
		return "", fmt.Errorf("invalid hex format")
	}

	// Decode hex to bytes
	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", fmt.Errorf("failed to decode hex: %s", err)
	}

	// Take first 32 bytes
	if len(decoded) > 32 {
		decoded = decoded[:32]
	}

	// Convert bytes to 5-bit groups
	var bits []byte
	for _, b := range decoded {
		for i := 7; i >= 0; i-- {
			bit := (b >> uint(i)) & 1
			bits = append(bits, bit)
		}
	}

	// Convert to base32
	const alphabet = "023456789acdefghjklmnpqrstuvwxyz"
	var encoded string
	for i := 0; i < len(bits); i += 5 {
		if i+5 <= len(bits) {
			value := byte(0)
			for j := 0; j < 5; j++ {
				if i+j < len(bits) {
					value |= bits[i+j] << uint(4-j)
				}
			}
			if int(value) < len(alphabet) {
				encoded += string(alphabet[value])
			}
		}
	}

	return "npub1" + encoded, nil
}

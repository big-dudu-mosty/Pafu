package main

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	maxPackFiles      = 60
	maxPackFileBytes  = 10 << 20  // 10 MB
	maxPackTotalBytes = 200 << 20 // 200 MB
)

var appSupportDirOverride string

const (
	appSupportDirName       = "Pafu"
	legacyAppSupportDirName = "Spank"
)

type packManifest struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Mode      string    `json:"mode"`
	Files     []string  `json:"files"`
	CreatedAt time.Time `json:"created_at"`
}

func appSupportPacksDir() (string, int, int, error) {
	if appSupportDirOverride != "" {
		dir := filepath.Join(appSupportDirOverride, "packs")
		migrateLegacyPacksDir(filepath.Join(appSupportDirOverride, "..", legacyAppSupportDirName, "packs"), dir, os.Getuid(), os.Getgid())
		return dir, os.Getuid(), os.Getgid(), nil
	}

	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" && sudoUser != "root" {
		u, err := user.Lookup(sudoUser)
		if err != nil {
			return "", -1, -1, fmt.Errorf("lookup sudo user %q: %w", sudoUser, err)
		}
		uid, _ := strconv.Atoi(u.Uid)
		gid, _ := strconv.Atoi(u.Gid)
		base := filepath.Join(u.HomeDir, "Library", "Application Support")
		dir := filepath.Join(base, appSupportDirName, "packs")
		migrateLegacyPacksDir(filepath.Join(base, legacyAppSupportDirName, "packs"), dir, uid, gid)
		return dir, uid, gid, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", -1, -1, err
	}
	base := filepath.Join(home, "Library", "Application Support")
	dir := filepath.Join(base, appSupportDirName, "packs")
	migrateLegacyPacksDir(filepath.Join(base, legacyAppSupportDirName, "packs"), dir, os.Getuid(), os.Getgid())
	return dir, os.Getuid(), os.Getgid(), nil
}

// migrateLegacyPacksDir moves any pre-Pafu user packs from the old
// "Application Support/Spank/packs" path to the new Pafu location. This is a
// best-effort migration: failures are silent so a missing legacy directory
// (the common case) does not bubble up as an error.
func migrateLegacyPacksDir(legacyDir, newDir string, uid, gid int) {
	if legacyDir == "" || newDir == "" || legacyDir == newDir {
		return
	}
	if _, err := os.Stat(legacyDir); err != nil {
		return
	}
	if _, err := os.Stat(newDir); err == nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(newDir), 0o755); err != nil {
		return
	}
	chownIfRoot(filepath.Dir(newDir), uid, gid)
	if err := os.Rename(legacyDir, newDir); err != nil {
		return
	}
	chownIfRoot(newDir, uid, gid)
}

func chownIfRoot(path string, uid, gid int) {
	if os.Geteuid() != 0 || uid < 0 || gid < 0 {
		return
	}
	_ = os.Chown(path, uid, gid)
}

func slugifyPackName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	re := regexp.MustCompile(`[^a-z0-9]+`)
	slug := strings.Trim(re.ReplaceAllString(name, "-"), "-")
	if slug == "" {
		return "pack"
	}
	if len(slug) > 32 {
		slug = strings.Trim(slug[:32], "-")
	}
	if slug == "" {
		return "pack"
	}
	return slug
}

func sanitizeMP3Name(name string, idx int) string {
	base := filepath.Base(name)
	ext := strings.ToLower(filepath.Ext(base))
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	stem = strings.Trim(re.ReplaceAllString(stem, "-"), "-._")
	if stem == "" {
		stem = "audio"
	}
	return fmt.Sprintf("%03d_%s%s", idx+1, stem, ext)
}

func loadUserPacks() (map[string]*soundPack, error) {
	root, _, _, err := appSupportPacksDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*soundPack{}, nil
		}
		return nil, err
	}

	packs := make(map[string]*soundPack)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		manifest, err := readPackManifest(dir)
		if err != nil {
			continue
		}
		if manifest.ID == "" || manifest.Name == "" || len(manifest.Files) == 0 {
			continue
		}

		files := make([]string, 0, len(manifest.Files))
		for _, file := range manifest.Files {
			if !strings.HasSuffix(strings.ToLower(file), ".mp3") {
				continue
			}
			fullPath := filepath.Join(dir, filepath.Base(file))
			if _, err := os.Stat(fullPath); err == nil {
				files = append(files, fullPath)
			}
		}
		sort.Strings(files)
		if len(files) == 0 {
			continue
		}

		packs[manifest.ID] = &soundPack{
			name:        "custom",
			displayName: manifest.Name,
			dir:         dir,
			mode:        modeRandom,
			files:       files,
			custom:      true,
			source:      "user",
			userManaged: true,
		}
	}
	return packs, nil
}

func readPackManifest(dir string) (packManifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "pack.json"))
	if err != nil {
		return packManifest{}, err
	}
	var manifest packManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return packManifest{}, err
	}
	return manifest, nil
}

func importUserPack(name string, headers []*multipart.FileHeader) (packInfo, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 40 {
		return packInfo{}, fmt.Errorf("pack name must be 1-40 characters")
	}
	if len(headers) == 0 {
		return packInfo{}, fmt.Errorf("at least one MP3 file is required")
	}
	if len(headers) > maxPackFiles {
		return packInfo{}, fmt.Errorf("too many files: max %d", maxPackFiles)
	}

	var total int64
	for _, header := range headers {
		if !strings.HasSuffix(strings.ToLower(header.Filename), ".mp3") {
			return packInfo{}, fmt.Errorf("only MP3 files are supported: %s", header.Filename)
		}
		if header.Size <= 0 {
			return packInfo{}, fmt.Errorf("empty file is not allowed: %s", header.Filename)
		}
		if header.Size > maxPackFileBytes {
			return packInfo{}, fmt.Errorf("file too large: %s", header.Filename)
		}
		total += header.Size
		if total > maxPackTotalBytes {
			return packInfo{}, fmt.Errorf("pack too large: max %d MB", maxPackTotalBytes>>20)
		}
	}

	root, uid, gid, err := appSupportPacksDir()
	if err != nil {
		return packInfo{}, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return packInfo{}, err
	}
	chownIfRoot(root, uid, gid)

	id := fmt.Sprintf("%s-%s", slugifyPackName(name), time.Now().Format("20060102150405"))
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return packInfo{}, err
	}
	chownIfRoot(dir, uid, gid)

	files := make([]string, 0, len(headers))
	savedFiles := make([]string, 0, len(headers))
	for idx, header := range headers {
		dstName := sanitizeMP3Name(header.Filename, idx)
		dstPath := filepath.Join(dir, dstName)
		if err := saveUploadedFile(header, dstPath); err != nil {
			_ = os.RemoveAll(dir)
			return packInfo{}, err
		}
		chownIfRoot(dstPath, uid, gid)
		files = append(files, dstPath)
		savedFiles = append(savedFiles, dstName)
	}

	manifest := packManifest{
		ID:        id,
		Name:      name,
		Mode:      "random",
		Files:     savedFiles,
		CreatedAt: time.Now(),
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = os.RemoveAll(dir)
		return packInfo{}, err
	}
	manifestPath := filepath.Join(dir, "pack.json")
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return packInfo{}, err
	}
	chownIfRoot(manifestPath, uid, gid)

	sort.Strings(files)
	pack := &soundPack{
		name:        "custom",
		displayName: name,
		dir:         dir,
		mode:        modeRandom,
		files:       files,
		custom:      true,
		source:      "user",
		userManaged: true,
	}
	registerPack(id, pack)
	info := toPackInfo(id, pack, false)
	publishBus("pack-imported", map[string]interface{}{"pack": info})
	return info, nil
}

func saveUploadedFile(header *multipart.FileHeader, dstPath string) error {
	src, err := header.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil {
		return err
	}
	if written > maxPackFileBytes {
		return fmt.Errorf("file too large: %s", header.Filename)
	}
	return nil
}

func deleteUserPack(id string) (packInfo, error) {
	packRegistryMu.RLock()
	pack, ok := packRegistry[id]
	packRegistryMu.RUnlock()
	if !ok {
		return packInfo{}, fmt.Errorf("pack not found: %s", id)
	}
	if !pack.userManaged {
		return packInfo{}, fmt.Errorf("pack cannot be deleted: %s", id)
	}

	info := toPackInfo(id, pack, id == activePackID)
	if err := os.RemoveAll(pack.dir); err != nil {
		return packInfo{}, err
	}
	unregisterPack(id)

	if info.Active {
		if _, err := activatePack("pain"); err != nil {
			return packInfo{}, err
		}
	}

	publishBus("pack-deleted", map[string]interface{}{
		"deleted_pack_id": id,
		"active_pack_id":  activePackID,
	})
	return info, nil
}
